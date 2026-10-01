package dep

import "testing"

func TestGraphDedupes(t *testing.T) {
	g := NewGraph("package-lock.json")
	a, b := Pkg{Npm, "a", "1.0.0"}, Pkg{Npm, "b", "2.0.0"}
	g.AddDirect(a)
	g.AddDirect(a)
	g.Add(a, b)
	g.Add(a, b)
	if len(g.Direct) != 1 || len(g.Deps[a]) != 1 {
		t.Fatalf("direct %v deps %v", g.Direct, g.Deps)
	}
	if ps := g.Packages(); len(ps) != 2 || ps[0] != a {
		t.Fatalf("packages %v", ps)
	}
}
