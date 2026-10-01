//go:build unix

package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestStartQueryStop runs a real graph-node. It is skipped unless
// DEPBLAST_TEST_GRAPH_NODE points at one.
func TestStartQueryStop(t *testing.T) {
	bin := os.Getenv("DEPBLAST_TEST_GRAPH_NODE")
	if bin == "" {
		t.Skip("DEPBLAST_TEST_GRAPH_NODE not set")
	}
	home := t.TempDir()
	// Run it through a link only this test uses, so pgrep below counts
	// this test's servers and no others.
	link := filepath.Join(home, "graph-node-under-test")
	if err := os.Symlink(bin, link); err != nil {
		t.Fatal(err)
	}
	bin = link
	t.Setenv("DEPBLAST_HOME", home)
	t.Setenv("DEPBLAST_GRAPH_NODE", bin)
	t.Setenv("DEPBLAST_HYDRA_URL", "")
	defer Stop(home)

	// Several commands starting together share one server.
	errs := make(chan error, 4)
	for range 4 {
		go func() {
			_, err := Connect(t.Context())
			errs <- err
		}()
	}
	for range 4 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	c, err := Connect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Query(t.Context(), "MATCH (p:Project) RETURN count(*) AS n", nil); err != nil {
		t.Fatal(err)
	}
	first, ok := Running(home)
	if !ok {
		t.Fatal("not running after Connect")
	}
	// A second Connect reuses the running server.
	if _, err := Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	if again, _ := Running(home); again.PID != first.PID {
		t.Fatalf("started a second server: %d then %d", first.PID, again.PID)
	}
	if out, _ := exec.Command("pgrep", "-f", bin).Output(); len(out) == 0 || len(splitLines(out)) != 1 {
		t.Fatalf("want one graph-node process, pgrep says %q", out)
	}
	if stopped, err := Stop(home); !stopped || err != nil {
		t.Fatalf("stop: %v %v", stopped, err)
	}
	if _, ok := Running(home); ok {
		t.Fatal("still running after Stop")
	}
}

func splitLines(b []byte) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}
