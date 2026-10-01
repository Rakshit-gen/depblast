package dep

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func crate(name, version string) Pkg { return Pkg{Cargo, name, version} }

func TestParseCargo(t *testing.T) {
	g := parseFile(t, func(n string, f *os.File) (*Graph, error) { return ParseCargo(n, f) }, "testdata/Cargo.lock")
	app := crate("app", "0.1.0")
	if !slices.Equal(g.Direct, []Pkg{app}) {
		t.Fatalf("direct %v", g.Direct)
	}
	if d := g.Deps[app]; !slices.Equal(d, []Pkg{crate("rand", "0.8.5"), crate("serde", "1.0.188")}) {
		t.Fatalf("app deps %v", d)
	}
	if d := g.Deps[crate("old-tool", "0.2.0")]; !slices.Equal(d, []Pkg{crate("rand", "0.7.3")}) {
		t.Fatalf("old-tool deps %v", d)
	}
	if len(g.Packages()) != 6 {
		t.Fatalf("packages %v", g.Packages())
	}
}

func TestParseCargoRejectsUnknownDependency(t *testing.T) {
	lock := "[[package]]\nname = \"app\"\nversion = \"0.1.0\"\ndependencies = [\"ghost\"]\n"
	if _, err := ParseCargo("Cargo.lock", strings.NewReader(lock)); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("err %v", err)
	}
}
