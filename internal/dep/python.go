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
		n := NormalizePy(p.Name)
		byName[n] = append(byName[n], Pkg{PyPI, n, p.Version})
	}
	find := func(ref uvRef) (Pkg, error) {
		for _, p := range byName[NormalizePy(ref.Name)] {
			if ref.Version == "" || p.Version == ref.Version {
				return p, nil
			}
		}
		return Pkg{}, fmt.Errorf("%s: dependency %s %s is not in the lockfile", name, ref.Name, ref.Version)
	}

	g := NewGraph(name)
	for _, p := range lock.Package {
		from := Pkg{PyPI, NormalizePy(p.Name), p.Version}
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

// NormalizePy applies PEP 503 name normalization, so Typing_Extensions
// and typing-extensions are the same package.
func NormalizePy(name string) string {
	return pySeparators.ReplaceAllString(strings.ToLower(name), "-")
}

type poetryLock struct {
	Package []struct {
		Name         string
		Version      string
		Dependencies map[string]any
	}
}

type pyproject struct {
	Project struct {
		Dependencies         []string
		OptionalDependencies map[string][]string `toml:"optional-dependencies"`
	}
	DependencyGroups map[string][]any `toml:"dependency-groups"`
	Tool             struct {
		Poetry struct {
			Dependencies    map[string]any
			DevDependencies map[string]any `toml:"dev-dependencies"`
			Group           map[string]struct {
				Dependencies map[string]any
			}
		}
	}
}

var pep508Name = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)`)

// names lists the packages a pyproject.toml asks for, from the [project]
// table, dependency groups, and Poetry's own tables.
func (p pyproject) names() []string {
	var out []string
	add := func(spec string) {
		if m := pep508Name.FindStringSubmatch(spec); m != nil {
			out = append(out, NormalizePy(m[1]))
		}
	}
	for _, s := range p.Project.Dependencies {
		add(s)
	}
	for _, group := range p.Project.OptionalDependencies {
		for _, s := range group {
			add(s)
		}
	}
	for _, group := range p.DependencyGroups {
		for _, s := range group {
			if s, ok := s.(string); ok { // tables are include-group entries
				add(s)
			}
		}
	}
	tables := []map[string]any{p.Tool.Poetry.Dependencies, p.Tool.Poetry.DevDependencies}
	for _, g := range p.Tool.Poetry.Group {
		tables = append(tables, g.Dependencies)
	}
	for _, t := range tables {
		for n := range t {
			if n != "python" {
				add(n)
			}
		}
	}
	return out
}

// ParsePoetry reads a poetry.lock. The lockfile doesn't say what the
// project asked for, so the pyproject.toml next to it is read for that
// (manifest may be nil); packages nothing else uses count as direct too.
// Poetry locks one version per package name.
func ParsePoetry(name string, lock io.Reader, manifest io.Reader) (*Graph, error) {
	var l poetryLock
	if _, err := toml.NewDecoder(lock).Decode(&l); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	byName := map[string]Pkg{}
	for _, p := range l.Package {
		n := NormalizePy(p.Name)
		byName[n] = Pkg{PyPI, n, p.Version}
	}
	g := NewGraph(name)
	used := map[Pkg]bool{}
	for _, p := range l.Package {
		from := byName[NormalizePy(p.Name)]
		g.touch(from)
		for dep := range p.Dependencies {
			// Optional dependencies whose extra wasn't requested are
			// not in the lockfile; skip them.
			if to, ok := byName[NormalizePy(dep)]; ok {
				g.Add(from, to)
				used[to] = true
			}
		}
	}
	if manifest != nil {
		var pp pyproject
		if _, err := toml.NewDecoder(manifest).Decode(&pp); err != nil {
			return nil, fmt.Errorf("%s: pyproject.toml: %w", name, err)
		}
		for _, n := range pp.names() {
			if p, ok := byName[n]; ok {
				g.AddDirect(p)
				used[p] = true
			}
		}
	}
	for _, p := range g.Packages() {
		if !used[p] {
			g.AddDirect(p)
		}
	}
	return g, nil
}
