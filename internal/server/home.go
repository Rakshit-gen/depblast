// Package server runs the HydraDB node depblast talks to. Unless told to
// use another server, depblast starts its own in the background the first
// time it is needed and leaves it running for the next command.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Home is depblast's folder: the bundled HydraDB, its data, its log and
// the token clients use. $DEPBLAST_HOME, or ~/.depblast.
func Home() (string, error) {
	if h := os.Getenv("DEPBLAST_HOME"); h != "" {
		return h, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".depblast"), nil
}

// token returns the bearer token for the local server, creating it on
// first use. Only the owner can read it.
func token(home string) (string, error) {
	path := filepath.Join(home, "token")
	b, err := os.ReadFile(path)
	if err == nil {
		return strings.TrimSpace(string(b)), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	raw := make([]byte, 32)
	rand.Read(raw)
	tok := hex.EncodeToString(raw)
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("writing token: %w", err)
	}
	return tok, nil
}

// graphNode finds the HydraDB binary: $DEPBLAST_GRAPH_NODE, then
// libexec/graph-node next to the depblast binary's folder (how the
// release archive is laid out), then the one in Home.
func graphNode(home string) (string, error) {
	if p := os.Getenv("DEPBLAST_GRAPH_NODE"); p != "" {
		return p, nil
	}
	var tried []string
	if exe, err := os.Executable(); err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			tried = append(tried, filepath.Join(filepath.Dir(exe), "..", "libexec", "graph-node"))
		}
	}
	tried = append(tried, filepath.Join(home, "libexec", "graph-node"))
	for _, p := range tried {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("HydraDB's graph-node was not found (looked in %s). Reinstall depblast, or set DEPBLAST_HYDRA_URL to use a running server", strings.Join(tried, ", "))
}
