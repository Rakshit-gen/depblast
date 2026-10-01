package dep

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

type npmLock struct {
	LockfileVersion int                   `json:"lockfileVersion"`
	Packages        map[string]npmPackage `json:"packages"`
	Dependencies    map[string]npmV1Entry `json:"dependencies"`
}

// npmV1Entry is the nested tree npm 5 and 6 wrote (lockfile version 1).
type npmV1Entry struct {
	Version      string                `json:"version"`
	Requires     map[string]string     `json:"requires"`
	Dependencies map[string]npmV1Entry `json:"dependencies"`
}

type npmPackage struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	Link                 bool              `json:"link"`
	Resolved             string            `json:"resolved"`
}

// ParseNpm reads a package-lock.json (lockfile version 2 or 3) or an
// npm-shrinkwrap.json. Each "packages" key is an install location like
// node_modules/a/node_modules/b, and a dependency is resolved the way
// Node resolves require(): the nearest node_modules folder going up.
// Workspace packages are counted as direct dependencies of the project.
func ParseNpm(name string, r io.Reader) (*Graph, error) {
	var lock npmLock
	if err := json.NewDecoder(r).Decode(&lock); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if lock.Packages == nil {
		if lock.Dependencies == nil {
			return nil, fmt.Errorf("%s: no \"packages\" or \"dependencies\" section", name)
		}
		lock.Packages = flattenV1(lock.Dependencies)
	}
	g := NewGraph(name)
	pkgAt := func(loc string) (Pkg, bool) {
		p, ok := lock.Packages[loc]
		if ok && p.Link {
			// A workspace package: node_modules/foo points at packages/foo.
			loc = p.Resolved
			p, ok = lock.Packages[loc]
		}
		if !ok || loc == "" {
			return Pkg{}, false
		}
		n := p.Name
		if n == "" {
			n = loc[strings.LastIndex(loc, "/")+1:]
			if i := strings.LastIndex(loc, "node_modules/"); i >= 0 {
				n = loc[i+len("node_modules/"):]
			}
		}
		return Pkg{Npm, n, p.Version}, true
	}

	for _, loc := range slices.Sorted(maps.Keys(lock.Packages)) {
		p := lock.Packages[loc]
		from, isPkg := pkgAt(loc)
		if loc != "" && !isPkg || p.Link {
			continue
		}
		inProject := !strings.Contains(loc, "node_modules/")
		if loc != "" && inProject {
			// A workspace package's own folder. It is part of the
			// project, so it counts as a direct dependency.
			g.AddDirect(from)
		}
		wants := []map[string]string{p.Dependencies, p.OptionalDependencies, p.PeerDependencies}
		if inProject {
			// npm installs dev dependencies of the root and of every
			// workspace, but not of packages from the registry.
			wants = append(wants, p.DevDependencies)
		}
		for _, want := range wants {
			for _, dep := range slices.Sorted(maps.Keys(want)) {
				to, ok := pkgAt(resolveNpm(lock.Packages, loc, dep))
				if !ok {
					continue // an optional or peer dependency that wasn't installed
				}
				if loc == "" {
					g.AddDirect(to)
				} else {
					g.Add(from, to)
				}
			}
		}
	}
	return g, nil
}

// resolveNpm finds where dep is installed as seen from the package at loc,
// or returns "" if it isn't installed anywhere visible.
func resolveNpm(pkgs map[string]npmPackage, loc, dep string) string {
	for {
		cand := "node_modules/" + dep
		if loc != "" {
			cand = loc + "/" + cand
		}
		if _, ok := pkgs[cand]; ok {
			return cand
		}
		if loc == "" {
			return ""
		}
		i := strings.LastIndex(loc, "node_modules/")
		if i < 0 {
			loc = "" // a workspace folder like packages/foo
			continue
		}
		loc = strings.TrimSuffix(loc[:i], "/")
	}
}

// flattenV1 turns a version 1 tree into the location map later versions
// use. Version 1 does not record what the project itself asked for, so the
// root's dependencies are approximated as the top level packages no other
// package resolves to. A package the project uses directly that is also pulled
// in by another one is then only reachable through that other package,
// which still says correctly that the project ships it.
func flattenV1(deps map[string]npmV1Entry) map[string]npmPackage {
	pkgs := map[string]npmPackage{}
	var walk func(prefix string, deps map[string]npmV1Entry)
	walk = func(prefix string, deps map[string]npmV1Entry) {
		for name, e := range deps {
			loc := prefix + "node_modules/" + name
			pkgs[loc] = npmPackage{Version: e.Version, Dependencies: e.Requires}
			walk(loc+"/", e.Dependencies)
		}
	}
	walk("", deps)
	used := map[string]bool{}
	for loc, p := range pkgs {
		for dep := range p.Dependencies {
			used[resolveNpm(pkgs, loc, dep)] = true
		}
	}
	root := map[string]string{}
	for name, e := range deps {
		if !used["node_modules/"+name] {
			root[name] = e.Version
		}
	}
	pkgs[""] = npmPackage{Dependencies: root}
	return pkgs
}
