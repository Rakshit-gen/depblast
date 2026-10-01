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
		return nil, fmt.Errorf("%s: lockfile version %d has no \"packages\" section", name, lock.LockfileVersion)
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
		if loc != "" && !strings.Contains(loc, "node_modules/") {
			// A workspace package's own folder. It is part of the
			// project, so it counts as a direct dependency.
			g.AddDirect(from)
		}
		wants := []map[string]string{p.Dependencies, p.OptionalDependencies, p.PeerDependencies}
		if loc == "" {
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
