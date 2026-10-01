// Command depblast finds which of your projects ship a given package
// version, and through which dependencies.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
)

const usage = `depblast finds which of your projects ship a package version, and why.

Usage:
  depblast index PATH...           read the lockfiles under PATH and store their dependency graphs
  depblast hit NAME[@VERSIONS]     list the projects that lock NAME, and the chain that pulls it in
  depblast projects                list the indexed lockfiles, marking changed and missing ones
  depblast forget PATH...          remove indexed lockfiles at or under PATH (-missing: all deleted ones)
  depblast server status|stop|log  manage the background HydraDB depblast runs

Supported lockfiles: package-lock.json, npm-shrinkwrap.json, yarn.lock (v1),
pnpm-lock.yaml (v6, v9), Cargo.lock, go.mod (needs Go installed), uv.lock,
poetry.lock.

Environment:
  DEPBLAST_HOME         where depblast keeps its data (default ~/.depblast)
  DEPBLAST_HYDRA_URL    use this HydraDB server instead of the bundled one
  DEPBLAST_HYDRA_TOKEN  bearer token for DEPBLAST_HYDRA_URL
`

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		fmt.Print(usage)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	commands := map[string]func(context.Context, []string) error{
		"index":    runIndex,
		"hit":      runHit,
		"projects": runProjects,
		"forget":   runForget,
		"server":   runServer,
	}
	run, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "depblast: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err := run(ctx, os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "depblast:", err)
		os.Exit(1)
	}
}

// rel shows a path relative to the working directory when it is inside
// it, which is shorter to read.
func rel(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	r, err := filepath.Rel(wd, path)
	if err != nil || strings.HasPrefix(r, "..") {
		return path
	}
	return r
}
