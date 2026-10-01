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
		if got := NormalizePy(in); got != want {
			t.Errorf("%s: %s", in, got)
		}
	}
}

func TestParsePoetry(t *testing.T) {
	lock, _ := os.Open("testdata/poetry.lock")
	defer lock.Close()
	manifest, _ := os.Open("testdata/pyproject.toml")
	defer manifest.Close()
	g, err := ParsePoetry("poetry.lock", lock, manifest)
	if err != nil {
		t.Fatal(err)
	}
	direct := slices.Clone(g.Direct)
	slices.SortFunc(direct, compare)
	// werkzeug is asked for directly as well as through flask.
	want := []Pkg{py("black", "23.11.0"), py("flask", "3.0.0"), py("requests", "2.31.0"), py("werkzeug", "3.0.1")}
	if !slices.Equal(direct, want) {
		t.Fatalf("direct %v", direct)
	}
	// Werkzeug is written with a capital W in flask's table; python-dotenv
	// is optional and not locked.
	if d := g.Deps[py("flask", "3.0.0")]; !slices.Equal(d, []Pkg{py("werkzeug", "3.0.1")}) {
		t.Fatalf("flask deps %v", d)
	}
}

func TestPyprojectNames(t *testing.T) {
	var p pyproject
	p.Project.Dependencies = []string{"requests[socks]>=2.31", "Django ~= 4.2", "pywin32; sys_platform == 'win32'"}
	p.DependencyGroups = map[string][]any{"test": {"pytest>=7", map[string]any{"include-group": "lint"}}}
	got := p.names()
	slices.Sort(got)
	if !slices.Equal(got, []string{"django", "pytest", "pywin32", "requests"}) {
		t.Fatalf("names %v", got)
	}
}
