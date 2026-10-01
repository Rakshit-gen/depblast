package dep

import (
	"os"
	"slices"
	"testing"
)

func py(name, version string) Pkg { return Pkg{PyPI, name, version} }

func TestParseUv(t *testing.T) {
	g := parseFile(t, func(n string, f *os.File) (*Graph, error) { return ParseUv(n, f) }, "testdata/uv.lock")
	app := py("app", "0.1.0")
	if !slices.Equal(g.Direct, []Pkg{app}) {
		t.Fatalf("direct %v", g.Direct)
	}
	if d := g.Deps[app]; !slices.Equal(d, []Pkg{py("requests", "2.31.0"), py("urllib3", "2.0.7"), py("pytest", "7.4.3")}) {
		t.Fatalf("app deps %v", d)
	}
	if d := g.Deps[py("requests", "2.31.0")]; !slices.Equal(d, []Pkg{py("certifi", "2023.7.22"), py("urllib3", "1.26.18")}) {
		t.Fatalf("requests deps %v", d)
	}
}

func TestNormalizePy(t *testing.T) {
	for in, want := range map[string]string{"Typing_Extensions": "typing-extensions", "zope.interface": "zope-interface", "a--b": "a-b"} {
		if got := normalizePy(in); got != want {
			t.Errorf("%s: %s", in, got)
		}
	}
}
