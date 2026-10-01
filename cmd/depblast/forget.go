package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rakshit-gen/depblast/internal/server"
	"github.com/Rakshit-gen/depblast/internal/store"
)

func runForget(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("forget", flag.ExitOnError)
	missing := fs.Bool("missing", false, "forget every indexed lockfile that no longer exists")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: depblast forget PATH...\n       depblast forget -missing\n\n"+
			"Removes indexed lockfiles at or under each PATH. Files on disk are not touched.\n")
	}
	fs.Parse(args)
	if fs.NArg() == 0 && !*missing {
		fs.Usage()
		os.Exit(2)
	}
	var roots []string
	for _, p := range fs.Args() {
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		roots = append(roots, abs)
	}
	c, err := server.Connect(ctx)
	if err != nil {
		return err
	}
	projects, err := store.Projects(ctx, c)
	if err != nil {
		return err
	}
	n := 0
	for _, p := range projects {
		if !under(p.Path, roots) {
			if _, err := os.Stat(p.Path); !*missing || err == nil {
				continue
			}
		}
		if err := store.Remove(ctx, c, p.Path); err != nil {
			return err
		}
		fmt.Println("forgot", rel(p.Path))
		n++
	}
	if n == 0 {
		fmt.Println("nothing to forget")
	}
	return nil
}

func under(path string, roots []string) bool {
	for _, r := range roots {
		if path == r || strings.HasPrefix(path, r+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
