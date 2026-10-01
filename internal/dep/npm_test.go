package dep

import (
	"os"
	"slices"
	"testing"
)

func parseFile(t *testing.T, parse func(string, *os.File) (*Graph, error), path string) *Graph {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := parse(path, f)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func npm(name, version string) Pkg { return Pkg{Npm, name, version} }

func TestParseNpmResolvesNestedCopies(t *testing.T) {
	g := parseFile(t, func(n string, f *os.File) (*Graph, error) { return ParseNpm(n, f) }, "testdata/package-lock.json")

	direct := slices.Clone(g.Direct)
	slices.SortFunc(direct, compare)
	want := []Pkg{npm("@babel/core", "7.22.0"), npm("express", "4.18.2"), npm("jest", "29.7.0")}
	if !slices.Equal(direct, want) {
		t.Fatalf("direct %v", direct)
	}
	// express sees the top level debug 2.6.9, @babel/core its own 4.3.4.
	if !slices.Contains(g.Deps[npm("express", "4.18.2")], npm("debug", "2.6.9")) {
		t.Fatalf("express deps %v", g.Deps[npm("express", "4.18.2")])
	}
	if d := g.Deps[npm("@babel/core", "7.22.0")]; !slices.Equal(d, []Pkg{npm("debug", "4.3.4")}) {
		t.Fatalf("@babel/core deps %v", d)
	}
	// The nested debug finds the ms next to it before the top level one.
	if d := g.Deps[npm("debug", "4.3.4")]; !slices.Equal(d, []Pkg{npm("ms", "2.1.2")}) {
		t.Fatalf("debug 4 deps %v", d)
	}
	// fsevents is optional and not installed, so it is skipped.
	if len(g.Deps[npm("jest", "29.7.0")]) != 0 || len(g.Packages()) != 8 {
		t.Fatalf("packages %v", g.Packages())
	}
}

func TestParseNpmFollowsWorkspaceLinks(t *testing.T) {
	g := parseFile(t, func(n string, f *os.File) (*Graph, error) { return ParseNpm(n, f) }, "testdata/workspaces-lock.json")
	web, util := npm("@acme/web", "0.3.0"), npm("@acme/util", "0.1.0")
	if len(g.Direct) != 2 || !slices.Contains(g.Direct, web) || !slices.Contains(g.Direct, util) {
		t.Fatalf("direct %v", g.Direct)
	}
	if d := g.Deps[web]; !slices.Equal(d, []Pkg{util, npm("react", "18.2.0")}) {
		t.Fatalf("web deps %v", d)
	}
	if d := g.Deps[util]; !slices.Equal(d, []Pkg{npm("lodash", "4.17.20")}) {
		t.Fatalf("util deps %v", d)
	}
}
