package dep

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

type uvRef struct {
	Name    string
	Version string
}

type uvLock struct {
	Package []struct {
		Name                 string
		Version              string
		Source               map[string]any
		Dependencies         []uvRef
		OptionalDependencies map[string][]uvRef `toml:"optional-dependencies"`
		DevDependencies      map[string][]uvRef `toml:"dev-dependencies"`
	}
}

// ParseUv reads a uv.lock. Packages whose source is the project folder
// (editable, virtual or a local directory) are the project's own and are
// counted as direct dependencies. Optional and dev dependency groups are
// included.
func ParseUv(name string, r io.Reader) (*Graph, error) {
	var lock uvLock
	if _, err := toml.NewDecoder(r).Decode(&lock); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	byName := map[string][]Pkg{}
	for _, p := range lock.Package {
		n := normalizePy(p.Name)
		byName[n] = append(byName[n], Pkg{PyPI, n, p.Version})
	}
	find := func(ref uvRef) (Pkg, error) {
		for _, p := range byName[normalizePy(ref.Name)] {
			if ref.Version == "" || p.Version == ref.Version {
				return p, nil
			}
		}
		return Pkg{}, fmt.Errorf("%s: dependency %s %s is not in the lockfile", name, ref.Name, ref.Version)
	}

	g := NewGraph(name)
	for _, p := range lock.Package {
		from := Pkg{PyPI, normalizePy(p.Name), p.Version}
		if p.Source["editable"] != nil || p.Source["virtual"] != nil || p.Source["directory"] != nil {
			g.AddDirect(from)
		}
		refs := p.Dependencies
		for _, group := range p.OptionalDependencies {
			refs = append(refs, group...)
		}
		for _, group := range p.DevDependencies {
			refs = append(refs, group...)
		}
		for _, ref := range refs {
			to, err := find(ref)
			if err != nil {
				return nil, err
			}
			g.Add(from, to)
		}
	}
	return g, nil
}

var pySeparators = regexp.MustCompile(`[-_.]+`)

// normalizePy applies PEP 503 name normalization, so Typing_Extensions
// and typing-extensions are the same package.
func normalizePy(name string) string {
	return pySeparators.ReplaceAllString(strings.ToLower(name), "-")
}
