package dep

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

type pnpmLock struct {
	LockfileVersion string                  `yaml:"lockfileVersion"`
	Importers       map[string]pnpmImporter `yaml:"importers"`
	Packages        map[string]pnpmSnapshot `yaml:"packages"`
	Snapshots       map[string]pnpmSnapshot `yaml:"snapshots"`
}

type pnpmImporter struct {
	Dependencies         map[string]pnpmRef `yaml:"dependencies"`
	DevDependencies      map[string]pnpmRef `yaml:"devDependencies"`
	OptionalDependencies map[string]pnpmRef `yaml:"optionalDependencies"`
}

type pnpmRef struct {
	Version string `yaml:"version"`
}

type pnpmSnapshot struct {
	Dependencies         map[string]string `yaml:"dependencies"`
	OptionalDependencies map[string]string `yaml:"optionalDependencies"`
}

// ParsePnpm reads a pnpm-lock.yaml, lockfile version 6 or 9. Every
// importer (the root and each workspace member) contributes direct
// dependencies. Packages are keyed by name@version, sometimes followed by
// the peers they were resolved against, like
// debug@4.3.4(supports-color@8.0.0); the peer part is dropped from the
// version but kept to find the right snapshot.
func ParsePnpm(name string, r io.Reader) (*Graph, error) {
	var lock pnpmLock
	if err := yaml.NewDecoder(r).Decode(&lock); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if !strings.HasPrefix(lock.LockfileVersion, "6") && !strings.HasPrefix(lock.LockfileVersion, "9") {
		return nil, fmt.Errorf("%s: pnpm lockfile version %q is not supported (6 and 9 are)", name, lock.LockfileVersion)
	}
	snaps := lock.Snapshots // version 9
	if snaps == nil {
		snaps = map[string]pnpmSnapshot{} // version 6 keys look like /debug@4.3.4
		for k, v := range lock.Packages {
			snaps[strings.TrimPrefix(k, "/")] = v
		}
	}

	g := NewGraph(name)
	for _, imp := range slices.Sorted(maps.Keys(lock.Importers)) {
		in := lock.Importers[imp]
		for _, deps := range []map[string]pnpmRef{in.Dependencies, in.DevDependencies, in.OptionalDependencies} {
			for _, dep := range slices.Sorted(maps.Keys(deps)) {
				if key, ok := pnpmKey(dep, deps[dep].Version); ok {
					g.AddDirect(pnpmPkg(key))
				}
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(snaps)) {
		s := snaps[key]
		from := pnpmPkg(key)
		g.touch(from)
		for _, deps := range []map[string]string{s.Dependencies, s.OptionalDependencies} {
			for _, dep := range slices.Sorted(maps.Keys(deps)) {
				if to, ok := pnpmKey(dep, deps[dep]); ok {
					g.Add(from, pnpmPkg(to))
				}
			}
		}
	}
	return g, nil
}

// pnpmKey turns a dependency name and the version pnpm recorded for it
// into a snapshot key. Workspace links ("link:../util") are skipped, and
// aliases record the real name@version as the version.
func pnpmKey(name, version string) (string, bool) {
	switch {
	case strings.HasPrefix(version, "link:"), strings.HasPrefix(version, "file:"):
		return "", false
	case strings.Contains(strings.SplitN(version, "(", 2)[0], "@"):
		return version, true
	}
	return name + "@" + version, true
}

func pnpmPkg(key string) Pkg {
	base := strings.SplitN(key, "(", 2)[0]
	i := strings.LastIndex(base, "@")
	return Pkg{Npm, base[:i], base[i+1:]}
}
