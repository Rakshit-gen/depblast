// Command evalnpm checks depblast's npm answers against npm's own. For each
// package-lock.json given, it indexes the lockfile, asks npm for its tree
// with `npm ls --all --package-lock-only --json` (nothing is installed),
// and for every package in the lockfile compares:
//
//   - reachable: depblast finds a path exactly when npm lists the package
//     under the project (extraneous entries are not reachable)
//   - depth: the shortest path depblast returns has npm's shortest depth
//   - links: every link in that path is a parent to child link npm shows
//
// It needs a HydraDB node, given by DEPBLAST_HYDRA_URL and
// DEPBLAST_HYDRA_TOKEN or started the same way depblast starts one.
//
//	go run ./scripts/evalnpm path/to/package-lock.json...
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rakshit-gen/depblast/internal/dep"
	"github.com/Rakshit-gen/depblast/internal/server"
	"github.com/Rakshit-gen/depblast/internal/store"
)

type lsNode struct {
	Version      string            `json:"version"`
	Resolved     string            `json:"resolved"`
	Extraneous   bool              `json:"extraneous"`
	Dependencies map[string]lsNode `json:"dependencies"`
}

// npmTree is npm's answer reduced to name@version: the links between
// packages and each package's shortest depth below the project.
type npmTree struct {
	links map[[2]string]bool
	depth map[string]int
}

func readNpm(dir string) (*npmTree, error) {
	cmd := exec.Command("npm", "ls", "--all", "--package-lock-only", "--json")
	cmd.Dir = dir
	// npm ls exits 1 when it reports problems such as extraneous
	// packages, and still prints the tree.
	out, _ := cmd.Output()
	var root lsNode
	if err := json.Unmarshal(out, &root); err != nil {
		return nil, fmt.Errorf("npm ls in %s: %w", dir, err)
	}
	t := &npmTree{links: map[[2]string]bool{}, depth: map[string]int{}}
	var walk func(parent string, n lsNode)
	walk = func(parent string, n lsNode) {
		for name, child := range n.Dependencies {
			if child.Version == "" || (parent == "" && child.Extraneous) {
				continue // missing optional dependency, or not reachable
			}
			id := realName(name, child.Resolved) + "@" + child.Version
			if parent != "" {
				t.links[[2]string{parent, id}] = true
			}
			walk(id, child)
		}
	}
	walk("", root)
	// npm ls prints each subtree once and later copies without children,
	// so depths come from a breadth-first pass over the links.
	next := map[string][]string{}
	for l := range t.links {
		next[l[0]] = append(next[l[0]], l[1])
	}
	var queue []string
	for name, child := range root.Dependencies {
		if child.Version != "" && !child.Extraneous {
			id := realName(name, child.Resolved) + "@" + child.Version
			if _, ok := t.depth[id]; !ok {
				t.depth[id] = 1
				queue = append(queue, id)
			}
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, c := range next[id] {
			if _, ok := t.depth[c]; !ok {
				t.depth[c] = t.depth[id] + 1
				queue = append(queue, c)
			}
		}
	}
	return t, nil
}

// realName undoes npm aliases. npm ls lists "typescript":
// "npm:@typescript/typescript6@6.0.2" under the alias, and depblast under
// the real name, which the registry URL still carries.
func realName(key, resolved string) string {
	rest, ok := strings.CutPrefix(resolved, "https://registry.npmjs.org/")
	if !ok {
		return key
	}
	name, _, ok := strings.Cut(rest, "/-/")
	if !ok {
		return key
	}
	return name
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/evalnpm package-lock.json...")
		os.Exit(2)
	}
	ctx := context.Background()
	c, err := server.Connect(ctx)
	if err != nil {
		fail(err)
	}
	fmt.Println("lockfile | packages | reachable agree | depth agree | links agree | too deep | path queries")
	fmt.Println("---|---|---|---|---|---|---")
	for _, arg := range os.Args[1:] {
		lock, err := filepath.Abs(arg)
		if err != nil {
			fail(err)
		}
		npm, err := readNpm(filepath.Dir(lock))
		if err != nil {
			fail(err)
		}
		g, err := dep.Read(ctx, lock)
		if err != nil {
			fail(err)
		}
		if err := store.Put(ctx, c, lock, g); err != nil {
			fail(err)
		}
		project := store.ProjectID(lock)
		var reach, depth, links, deep, total int
		var spent time.Duration
		for _, p := range g.Packages() {
			total++
			start := time.Now()
			paths, err := store.Paths(ctx, c, project, store.PkgID(project, p), 1)
			spent += time.Since(start)
			if err != nil {
				fail(err)
			}
			want, listed := npm.depth[p.String()]
			if len(paths) == 0 && listed && want > store.MaxDepth {
				deep++
				continue
			}
			if (len(paths) > 0) != listed {
				fmt.Fprintf(os.Stderr, "%s: %s reachable: depblast %v, npm %v\n", arg, p, len(paths) > 0, listed)
				continue
			}
			reach++
			if !listed {
				depth++
				links++
				continue
			}
			chain := paths[0]
			if len(chain) == want {
				depth++
			} else {
				fmt.Fprintf(os.Stderr, "%s: %s depth: depblast %d, npm %d\n", arg, p, len(chain), want)
			}
			ok := true
			for i := 1; i < len(chain); i++ {
				if !npm.links[[2]string{chain[i-1].String(), chain[i].String()}] {
					fmt.Fprintf(os.Stderr, "%s: link %s > %s not in npm's tree\n", arg, chain[i-1], chain[i])
					ok = false
				}
			}
			if ok {
				links++
			}
		}
		if err := store.Remove(ctx, c, lock); err != nil {
			fail(err)
		}
		fmt.Printf("%s | %d | %d | %d | %d | %d | %.1f ms avg\n", arg, total, reach, depth, links, deep,
			float64(spent.Microseconds())/1000/float64(total))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "evalnpm:", err)
	os.Exit(1)
}
