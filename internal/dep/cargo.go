package dep

import (
	"fmt"
	"io"
	"strings"

	"github.com/BurntSushi/toml"
)

type cargoLock struct {
	Package []struct {
		Name         string
		Version      string
		Source       string
		Dependencies []string
	}
}

// ParseCargo reads a Cargo.lock. Packages without a source are the
// project's own crates (one, or every member of a workspace); they are
// counted as direct dependencies so paths start at the crate that pulls
// a package in.
func ParseCargo(name string, r io.Reader) (*Graph, error) {
	var lock cargoLock
	if _, err := toml.NewDecoder(r).Decode(&lock); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	// A dependency is written "name" when only one version is locked,
	// otherwise "name version" or "name version (source)".
	byName := map[string][]Pkg{}
	for _, p := range lock.Package {
		byName[p.Name] = append(byName[p.Name], Pkg{Cargo, p.Name, p.Version})
	}
	find := func(ref string) (Pkg, error) {
		f := strings.Fields(ref)
		for _, p := range byName[f[0]] {
			if len(f) == 1 || p.Version == f[1] {
				return p, nil
			}
		}
		return Pkg{}, fmt.Errorf("%s: dependency %q is not in the lockfile", name, ref)
	}

	g := NewGraph(name)
	for _, p := range lock.Package {
		from := Pkg{Cargo, p.Name, p.Version}
		if p.Source == "" {
			g.AddDirect(from)
		}
		for _, ref := range p.Dependencies {
			to, err := find(ref)
			if err != nil {
				return nil, err
			}
			g.Add(from, to)
		}
	}
	return g, nil
}
