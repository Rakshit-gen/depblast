package dep

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

type yarnEntry struct {
	version string
	deps    []string // descriptors like "debug@^4.1.0"
}

// ParseYarn reads a classic (v1) yarn.lock. The lockfile maps
// descriptors like "debug@^4.1.0" to a resolved version but doesn't say
// what the project itself asked for, so the package.json next to it is
// read for that (manifest may be nil). Entries nothing else uses are
// counted as direct too, which covers workspace members' dependencies.
func ParseYarn(name string, lock io.Reader, manifest io.Reader) (*Graph, error) {
	entries, err := readYarnV1(lock)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	g := NewGraph(name)
	pkgOf := map[*yarnEntry]Pkg{}
	for desc, e := range entries {
		pkgOf[e] = Pkg{Npm, descName(desc), e.version}
	}
	used := map[*yarnEntry]bool{}
	for _, desc := range slices.Sorted(maps.Keys(entries)) {
		e := entries[desc]
		for _, d := range e.deps {
			if to, ok := entries[d]; ok {
				g.Add(pkgOf[e], pkgOf[to])
				used[to] = true
			}
		}
	}
	if manifest != nil {
		var m struct {
			Dependencies, DevDependencies, OptionalDependencies map[string]string
		}
		if err := json.NewDecoder(manifest).Decode(&m); err != nil {
			return nil, fmt.Errorf("%s: package.json: %w", name, err)
		}
		for _, want := range []map[string]string{m.Dependencies, m.DevDependencies, m.OptionalDependencies} {
			for _, n := range slices.Sorted(maps.Keys(want)) {
				if e, ok := entries[n+"@"+want[n]]; ok {
					g.AddDirect(pkgOf[e])
					used[e] = true
				}
			}
		}
	}
	for _, desc := range slices.Sorted(maps.Keys(entries)) {
		if e := entries[desc]; !used[e] {
			g.AddDirect(pkgOf[e])
			used[e] = true
		}
	}
	return g, nil
}

// descName returns the package name of a descriptor: "@babel/core" from
// "@babel/core@^7.0.0".
func descName(desc string) string {
	i := strings.LastIndex(desc, "@")
	if i <= 0 {
		return desc
	}
	return desc[:i]
}

// readYarnV1 parses the indentation based v1 format into a map from every
// descriptor to its entry. Descriptors listed together share one entry.
func readYarnV1(r io.Reader) (map[string]*yarnEntry, error) {
	entries := map[string]*yarnEntry{}
	var cur *yarnEntry
	inDeps := false
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		switch {
		case indent == 0:
			if !strings.HasSuffix(trimmed, ":") {
				return nil, fmt.Errorf("line %d: expected an entry header", n)
			}
			cur = &yarnEntry{}
			inDeps = false
			for _, d := range strings.Split(strings.TrimSuffix(trimmed, ":"), ",") {
				entries[unquote(strings.TrimSpace(d))] = cur
			}
		case cur == nil:
			return nil, fmt.Errorf("line %d: field outside an entry", n)
		case indent == 2:
			key, val, _ := strings.Cut(trimmed, " ")
			inDeps = key == "dependencies:" || key == "optionalDependencies:"
			if key == "version" {
				cur.version = unquote(val)
			}
		case indent == 4 && inDeps:
			dep, rng, _ := strings.Cut(trimmed, " ")
			cur.deps = append(cur.deps, unquote(dep)+"@"+unquote(rng))
		}
	}
	return entries, sc.Err()
}

func unquote(s string) string { return strings.Trim(s, `"`) }
