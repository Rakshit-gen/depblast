package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenIsCreatedOncePrivately(t *testing.T) {
	home := filepath.Join(t.TempDir(), "depblast")
	a, err := token(home)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := token(home)
	if a != b || len(a) != 64 {
		t.Fatalf("tokens %q %q", a, b)
	}
	st, _ := os.Stat(filepath.Join(home, "token"))
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}
}

func TestGraphNodeLookup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DEPBLAST_GRAPH_NODE", "")
	if _, err := graphNode(home); err == nil {
		t.Fatal("found a binary that isn't there")
	}
	bin := filepath.Join(home, "libexec", "graph-node")
	os.MkdirAll(filepath.Dir(bin), 0o755)
	os.WriteFile(bin, nil, 0o755)
	if got, err := graphNode(home); err != nil || got != bin {
		t.Fatalf("got %q %v", got, err)
	}
	t.Setenv("DEPBLAST_GRAPH_NODE", "/opt/graph-node")
	if got, _ := graphNode(home); got != "/opt/graph-node" {
		t.Fatalf("env ignored: %q", got)
	}
}
