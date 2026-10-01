package dep

import (
	"os"
	"slices"
	"testing"
)

func TestParseYarn(t *testing.T) {
	lock, _ := os.Open("testdata/yarn.lock")
	defer lock.Close()
	manifest, _ := os.Open("testdata/yarn-package.json")
	defer manifest.Close()
	g, err := ParseYarn("yarn.lock", lock, manifest)
	if err != nil {
		t.Fatal(err)
	}
	direct := slices.Clone(g.Direct)
	slices.SortFunc(direct, compare)
	// debug 4.3.4 is both asked for by the project and used by
	// @babel/core. left-pad is used by nothing, so it is counted as direct.
	want := []Pkg{npm("@babel/core", "7.22.0"), npm("debug", "4.3.4"), npm("express", "4.18.2"), npm("left-pad", "1.3.0")}
	if !slices.Equal(direct, want) {
		t.Fatalf("direct %v", direct)
	}
	if d := g.Deps[npm("express", "4.18.2")]; !slices.Equal(d, []Pkg{npm("debug", "2.6.9")}) {
		t.Fatalf("express deps %v", d)
	}
	if d := g.Deps[npm("debug", "4.3.4")]; !slices.Equal(d, []Pkg{npm("ms", "2.1.2")}) {
		t.Fatalf("debug deps %v", d)
	}
}

func TestDescName(t *testing.T) {
	for in, want := range map[string]string{"@babel/core@^7.0.0": "@babel/core", "ms@2.0.0": "ms", "left-pad": "left-pad"} {
		if got := descName(in); got != want {
			t.Errorf("%s: %s", in, got)
		}
	}
}
