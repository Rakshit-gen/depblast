package dep

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestParsePnpm9(t *testing.T) {
	g := parseFile(t, func(n string, f *os.File) (*Graph, error) { return ParsePnpm(n, f) }, "testdata/pnpm-lock.yaml")
	direct := slices.Clone(g.Direct)
	slices.SortFunc(direct, compare)
	// The workspace link is skipped; the alias is read as strip-ansi; the
	// member's debug drops its peer suffix.
	want := []Pkg{npm("debug", "4.3.4"), npm("express", "4.18.2"), npm("strip-ansi", "6.0.1")}
	if !slices.Equal(direct, want) {
		t.Fatalf("direct %v", direct)
	}
	if d := g.Deps[npm("debug", "4.3.4")]; !slices.Equal(d, []Pkg{npm("ms", "2.1.2"), npm("supports-color", "8.0.0")}) {
		t.Fatalf("debug deps %v", d)
	}
	if d := g.Deps[npm("express", "4.18.2")]; !slices.Equal(d, []Pkg{npm("debug", "2.6.9")}) {
		t.Fatalf("express deps %v", d)
	}
}

func TestParsePnpm6(t *testing.T) {
	lock := `lockfileVersion: '6.0'
importers:
  .:
    dependencies:
      '@scope/pkg':
        specifier: ^1.0.0
        version: 1.2.0
packages:
  /@scope/pkg@1.2.0:
    dependencies:
      ms: 2.1.2
  /ms@2.1.2:
    resolution: {integrity: sha512-x}
`
	g, err := ParsePnpm("pnpm-lock.yaml", strings.NewReader(lock))
	if err != nil {
		t.Fatal(err)
	}
	scoped := npm("@scope/pkg", "1.2.0")
	if !slices.Equal(g.Direct, []Pkg{scoped}) || !slices.Equal(g.Deps[scoped], []Pkg{npm("ms", "2.1.2")}) {
		t.Fatalf("direct %v deps %v", g.Direct, g.Deps)
	}
}

func TestParsePnpmRejectsOldVersions(t *testing.T) {
	if _, err := ParsePnpm("pnpm-lock.yaml", strings.NewReader("lockfileVersion: 5.4\n")); err == nil {
		t.Fatal("version 5.4 accepted")
	}
}
