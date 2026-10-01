package dep

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ParseGoGraph builds a graph from the output of `go mod graph` and
// `go list -m all`. The first lists every requirement of every module
// version Go looked at, including versions that lost to a newer one; the
// second says which version of each module the build actually uses. Only
// selected versions are kept, and each edge points at the selected
// version of its target, which is what ends up in the binary.
func ParseGoGraph(name string, modGraph, listAll io.Reader) (*Graph, error) {
	selected := map[string]string{}
	main := ""
	sc := bufio.NewScanner(listAll)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		switch {
		case len(f) == 1 && main == "":
			main = f[0]
		case len(f) >= 2:
			selected[f[0]] = f[1]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if main == "" {
		return nil, fmt.Errorf("%s: go list -m all named no main module", name)
	}

	g := NewGraph(name)
	pick := func(mod string) (Pkg, bool) {
		path, version, _ := strings.Cut(mod, "@")
		if v, ok := selected[path]; ok && (version == "" || version == v) {
			return Pkg{Go, path, v}, true
		}
		return Pkg{}, false
	}
	sc = bufio.NewScanner(modGraph)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 2 {
			continue
		}
		toPath, _, _ := strings.Cut(f[1], "@")
		to, ok := pick(toPath)
		if !ok {
			continue
		}
		fromPath, _, _ := strings.Cut(f[0], "@")
		if fromPath == main && !strings.Contains(f[0], "@") {
			g.AddDirect(to)
			continue
		}
		if from, ok := pick(f[0]); ok {
			g.Add(from, to)
		}
	}
	return g, sc.Err()
}

// GoGraph runs the go tool in the module folder holding gomodPath. It
// needs Go installed, and it may download the go.mod files of
// dependencies that aren't in the module cache yet.
func GoGraph(ctx context.Context, gomodPath string) (*Graph, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return nil, fmt.Errorf("%s: reading Go modules needs the go command, which is not on PATH", gomodPath)
	}
	dir := filepath.Dir(gomodPath)
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		// Vendored or not, read the module graph rather than vendor/.
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("%s: go %s: %v: %s", gomodPath, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}
	graph, err := run("mod", "graph")
	if err != nil {
		return nil, err
	}
	list, err := run("list", "-m", "all")
	if err != nil {
		return nil, err
	}
	return ParseGoGraph(gomodPath, bytes.NewReader(graph), bytes.NewReader(list))
}
