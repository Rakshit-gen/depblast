package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Rakshit-gen/depblast/internal/dep"
	"github.com/Rakshit-gen/depblast/internal/server"
	"github.com/Rakshit-gen/depblast/internal/store"
	"github.com/Rakshit-gen/depblast/internal/version"
)

func runHit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("hit", flag.ExitOnError)
	paths := fs.Int("paths", 1, "how many dependency chains to show per project, shortest first")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: depblast hit [flags] NAME[@VERSIONS]\n\n"+
			"Lists the indexed projects that lock a version of NAME, with the chain of\n"+
			"dependencies that pulls it in. VERSIONS is an exact version or a range:\n\n"+
			"  depblast hit lodash@4.17.20\n"+
			"  depblast hit 'lodash@<4.17.21'\n"+
			"  depblast hit 'golang.org/x/net@>=v0.10.0 <v0.17.0'\n"+
			"  depblast hit @babel/traverse\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	name, spec := splitSpec(fs.Arg(0))
	rng, err := version.ParseRange(spec)
	if err != nil {
		return err
	}

	c, err := server.Connect(ctx)
	if err != nil {
		return err
	}
	copies, err := store.Copies(ctx, c, name)
	if err != nil {
		return err
	}
	copies = slices.DeleteFunc(copies, func(cp store.Copy) bool { return !rng.Match(cp.Pkg.Version) })
	if len(copies) == 0 {
		fmt.Printf("no indexed project locks %s\n", fs.Arg(0))
		return nil
	}
	projects, err := store.Projects(ctx, c)
	if err != nil {
		return err
	}
	pathOf := map[uint64]string{}
	for _, p := range projects {
		pathOf[p.ID] = p.Path
	}
	slices.SortFunc(copies, func(a, b store.Copy) int {
		if c := strings.Compare(pathOf[a.Project], pathOf[b.Project]); c != 0 {
			return c
		}
		return version.Compare(a.Pkg.Version, b.Pkg.Version)
	})

	nProjects := 0
	for i, cp := range copies {
		if i == 0 || cp.Project != copies[i-1].Project {
			nProjects++
			fmt.Printf("\n%s\n", rel(pathOf[cp.Project]))
		}
		chains, err := store.Paths(ctx, c, cp.Project, cp.ID, *paths)
		if err != nil {
			return err
		}
		if len(chains) == 0 {
			fmt.Printf("  %s (locked, but not reachable within %d links of the project)\n", cp.Pkg, store.MaxDepth)
		}
		for _, chain := range chains {
			fmt.Printf("  %s\n", formatChain(chain))
		}
	}
	fmt.Printf("\n%d copies of %s in %d of %d projects\n", len(copies), name, nProjects, len(projects))
	return nil
}

// splitSpec splits "name@versions" at the last @ that isn't the first
// character, so "@babel/core@7.0.0" keeps its scope.
func splitSpec(s string) (name, versions string) {
	if i := strings.LastIndex(s, "@"); i > 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func formatChain(chain []dep.Pkg) string {
	parts := make([]string, len(chain))
	for i, p := range chain {
		parts[i] = p.Name + " " + p.Version
	}
	return strings.Join(parts, " > ")
}
