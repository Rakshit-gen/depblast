package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/Rakshit-gen/depblast/internal/dep"
	"github.com/Rakshit-gen/depblast/internal/server"
	"github.com/Rakshit-gen/depblast/internal/store"
)

func runIndex(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: depblast index PATH...\n\n"+
			"Finds the lockfiles under each PATH (a folder or a lockfile) and stores their\n"+
			"dependency graphs. Indexing a lockfile again replaces what was stored for it.\n")
	}
	fs.Parse(args)
	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(2)
	}
	var files []string
	for _, p := range fs.Args() {
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return err
		}
		if !st.IsDir() {
			files = append(files, abs)
			continue
		}
		found, err := dep.Find(abs)
		if err != nil {
			return err
		}
		files = append(files, found...)
	}
	if len(files) == 0 {
		return errors.New("no lockfiles found")
	}

	c, err := server.Connect(ctx)
	if err != nil {
		return err
	}
	start := time.Now()
	graphs, errs := readAll(ctx, files)
	failed, pkgs := 0, 0
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for i, f := range files {
		if errs[i] != nil {
			fmt.Fprintf(os.Stderr, "skipped %s: %v\n", rel(f), errs[i])
			failed++
			continue
		}
		if err := store.Put(ctx, c, f, graphs[i]); err != nil {
			return fmt.Errorf("storing %s: %w", rel(f), err)
		}
		pkgs += len(graphs[i].Deps)
		fmt.Fprintf(w, "%s\t%d packages\n", rel(f), len(graphs[i].Deps))
	}
	w.Flush()
	fmt.Printf("indexed %s, %s in %.1fs\n", plural(len(files)-failed, "lockfile", "lockfiles"), plural(pkgs, "package", "packages"), time.Since(start).Seconds())
	if failed > 0 {
		return fmt.Errorf("%s could not be read", plural(failed, "lockfile", "lockfiles"))
	}
	return nil
}

// readAll parses lockfiles four at a time; Go modules run the go tool,
// which is slow enough to be worth overlapping.
func readAll(ctx context.Context, files []string) ([]*dep.Graph, []error) {
	graphs := make([]*dep.Graph, len(files))
	errs := make([]error, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, f := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			graphs[i], errs[i] = dep.Read(ctx, f)
		}()
	}
	wg.Wait()
	return graphs, errs
}
