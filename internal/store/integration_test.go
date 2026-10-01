package store

import (
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/Rakshit-gen/depblast/internal/dep"
	"github.com/Rakshit-gen/depblast/internal/hydra"
)

// TestAgainstRealHydraDB stores two small projects in a live server and
// reads them back. It is skipped unless DEPBLAST_TEST_HYDRA_URL and
// DEPBLAST_TEST_HYDRA_TOKEN are set.
func TestAgainstRealHydraDB(t *testing.T) {
	url := os.Getenv("DEPBLAST_TEST_HYDRA_URL")
	if url == "" {
		t.Skip("DEPBLAST_TEST_HYDRA_URL not set")
	}
	c := hydra.New(url, os.Getenv("DEPBLAST_TEST_HYDRA_TOKEN"))
	ctx := t.Context()
	run := fmt.Sprint(time.Now().UnixNano())
	pkg := func(name string) dep.Pkg { return dep.Pkg{Eco: dep.Npm, Name: name + "-" + run, Version: "1.0.0"} }
	a, b, cc, d, e, target := pkg("a"), pkg("b"), pkg("c"), pkg("d"), pkg("e"), pkg("target")

	// web reaches target in two links (a, target) and in four (b, c, d, target).
	web := dep.NewGraph("/test/" + run + "/web/package-lock.json")
	web.AddDirect(a)
	web.AddDirect(b)
	web.Add(a, target)
	web.Add(b, cc)
	web.Add(cc, d)
	web.Add(d, target)
	api := dep.NewGraph("/test/" + run + "/api/package-lock.json")
	api.AddDirect(e)
	for _, g := range []*dep.Graph{web, api} {
		if err := Put(ctx, c, g.Lockfile, g); err != nil {
			t.Fatal(err)
		}
		defer Remove(ctx, c, g.Lockfile)
	}

	all, err := Distinct(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	mine := 0
	for _, p := range all {
		if slices.Contains([]dep.Pkg{a, b, cc, d, e, target}, p) {
			mine++
		}
	}
	if mine != 6 {
		t.Fatalf("Distinct found %d of this run's 6 packages", mine)
	}

	copies, err := Copies(ctx, c, target.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 1 || copies[0].Project != ProjectID(web.Lockfile) || copies[0].Pkg != target {
		t.Fatalf("copies %+v", copies)
	}
	paths, err := Paths(ctx, c, copies[0].Project, copies[0].ID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || !slices.Equal(paths[0], []dep.Pkg{a, target}) || !slices.Equal(paths[1], []dep.Pkg{b, cc, d, target}) {
		t.Fatalf("paths %v", paths)
	}

	// Storing again replaces rather than adds.
	web.Deps[a] = nil
	if err := Put(ctx, c, web.Lockfile, web); err != nil {
		t.Fatal(err)
	}
	if paths, _ := Paths(ctx, c, copies[0].Project, copies[0].ID, 5); len(paths) != 1 {
		t.Fatalf("after re-put %v", paths)
	}
	if err := Remove(ctx, c, api.Lockfile); err != nil {
		t.Fatal(err)
	}
	if copies, _ := Copies(ctx, c, e.Name); len(copies) != 0 {
		t.Fatalf("removed project still has %v", copies)
	}
}
