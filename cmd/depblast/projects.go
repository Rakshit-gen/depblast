package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/Rakshit-gen/depblast/internal/server"
	"github.com/Rakshit-gen/depblast/internal/store"
)

func runProjects(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("projects", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: depblast projects\n\n"+
			"Lists the indexed lockfiles. A lockfile changed since it was indexed is marked\n"+
			"\"changed\"; run depblast index on it again. One that no longer exists is\n"+
			"marked \"missing\"; depblast forget removes it.\n")
	}
	fs.Parse(args)
	c, err := server.Connect(ctx)
	if err != nil {
		return err
	}
	projects, err := store.Projects(ctx, c)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		fmt.Println("nothing indexed yet, run: depblast index PATH")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	for _, p := range projects {
		note := ""
		if st, err := os.Stat(p.Path); err != nil {
			note = "  missing"
		} else if st.ModTime().After(p.Indexed) {
			note = "  changed"
		}
		fmt.Fprintf(w, "%s\t%d packages\tindexed %s%s\n", rel(p.Path), p.Packages, ago(p.Indexed), note)
	}
	return nil
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
