// Package dep holds the dependency graph read from one lockfile, and the
// parsers that read each lockfile format.
package dep

import (
	"slices"
	"strings"
)

// Ecosystem names follow OSV.dev, so packages can be looked up there as is.
const (
	Npm   = "npm"
	Cargo = "crates.io"
	Go    = "Go"
	PyPI  = "PyPI"
)

// Pkg is one resolved package version.
type Pkg struct {
	Eco     string
	Name    string
	Version string
}

func (p Pkg) String() string { return p.Name + "@" + p.Version }

// Graph is what one lockfile says: the project's direct dependencies, and
// for each package the versions it pulled in.
type Graph struct {
	Lockfile string
	Direct   []Pkg
	Deps     map[Pkg][]Pkg
}

func NewGraph(lockfile string) *Graph {
	return &Graph{Lockfile: lockfile, Deps: map[Pkg][]Pkg{}}
}

// AddDirect records a dependency of the project itself.
func (g *Graph) AddDirect(p Pkg) {
	if !slices.Contains(g.Direct, p) {
		g.Direct = append(g.Direct, p)
	}
	g.touch(p)
}

// Add records that from depends on to.
func (g *Graph) Add(from, to Pkg) {
	g.touch(from)
	g.touch(to)
	if !slices.Contains(g.Deps[from], to) {
		g.Deps[from] = append(g.Deps[from], to)
	}
}

func (g *Graph) touch(p Pkg) {
	if _, ok := g.Deps[p]; !ok {
		g.Deps[p] = nil
	}
}

// Packages lists every package in the graph, sorted.
func (g *Graph) Packages() []Pkg {
	out := make([]Pkg, 0, len(g.Deps))
	for p := range g.Deps {
		out = append(out, p)
	}
	slices.SortFunc(out, compare)
	return out
}

func compare(a, b Pkg) int {
	if c := strings.Compare(a.Name, b.Name); c != 0 {
		return c
	}
	return strings.Compare(a.Version, b.Version)
}
