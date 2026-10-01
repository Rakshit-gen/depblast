//go:build unix

package server

import (
	"os"
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
	t.Setenv("DEPBLAST_HOME", home)
	t.Setenv("DEPBLAST_GRAPH_NODE", bin)
	t.Setenv("DEPBLAST_HYDRA_URL", "")
	defer Stop(home)

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
	if stopped, err := Stop(home); !stopped || err != nil {
		t.Fatalf("stop: %v %v", stopped, err)
	}
	if _, ok := Running(home); ok {
		t.Fatal("still running after Stop")
	}
}
