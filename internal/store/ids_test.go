package store

import (
	"testing"

	"github.com/Rakshit-gen/depblast/internal/dep"
)

func TestIDs(t *testing.T) {
	lodash := dep.Pkg{Eco: dep.Npm, Name: "lodash", Version: "4.17.20"}
	a, b := ProjectID("/code/a/package-lock.json"), ProjectID("/code/b/package-lock.json")
	if a == b || PkgID(a, lodash) == PkgID(b, lodash) {
		t.Fatal("projects share ids")
	}
	if PkgID(a, lodash) != PkgID(a, lodash) {
		t.Fatal("ids not stable")
	}
	// Name and version must not run together: "ab"+"c" vs "a"+"bc".
	if PkgID(a, dep.Pkg{Eco: dep.Npm, Name: "ab", Version: "c"}) == PkgID(a, dep.Pkg{Eco: dep.Npm, Name: "a", Version: "bc"}) {
		t.Fatal("ambiguous id")
	}
	for _, id := range []uint64{a, b, PkgID(a, lodash), edgeID(a, b)} {
		if id >= 1<<63 {
			t.Fatalf("id %d too large", id)
		}
	}
}
