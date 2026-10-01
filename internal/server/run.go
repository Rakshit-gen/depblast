//go:build unix

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Rakshit-gen/depblast/internal/hydra"
)

// State is what a running local server left in Home/server.json.
type State struct {
	PID   int    `json:"pid"`
	HTTP  string `json:"http"`
	Admin string `json:"admin"`
	Bolt  string `json:"bolt"`
}

// Connect returns a client for the HydraDB server depblast should use:
// the one in $DEPBLAST_HYDRA_URL (with $DEPBLAST_HYDRA_TOKEN) if set,
// otherwise the local one, started first if it isn't running.
func Connect(ctx context.Context) (*hydra.Client, error) {
	if url := os.Getenv("DEPBLAST_HYDRA_URL"); url != "" {
		return hydra.New(url, os.Getenv("DEPBLAST_HYDRA_TOKEN")), nil
	}
	home, err := Home()
	if err != nil {
		return nil, err
	}
	tok, err := token(home)
	if err != nil {
		return nil, err
	}
	st, ok := Running(home)
	if !ok {
		if st, err = start(ctx, home); err != nil {
			return nil, err
		}
	}
	return hydra.New("http://"+st.HTTP, tok), nil
}

// Running reports the local server's state if it is up and ready.
func Running(home string) (State, bool) {
	var st State
	b, err := os.ReadFile(filepath.Join(home, "server.json"))
	if err != nil || json.Unmarshal(b, &st) != nil {
		return st, false
	}
	if syscall.Kill(st.PID, 0) != nil {
		return st, false
	}
	return st, ready(st.Admin)
}

func ready(admin string) bool {
	c := http.Client{Timeout: time.Second}
	resp, err := c.Get("http://" + admin + "/readyz")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func start(ctx context.Context, home string) (State, error) {
	bin, err := graphNode(home)
	if err != nil {
		return State{}, err
	}
	data := filepath.Join(home, "data")
	for _, d := range []string{"store", "cache"} {
		if err := os.MkdirAll(filepath.Join(data, d), 0o700); err != nil {
			return State{}, err
		}
	}
	ports, err := freePorts(3)
	if err != nil {
		return State{}, err
	}
	st := State{HTTP: ports[0], Admin: ports[1], Bolt: ports[2]}
	logPath := filepath.Join(home, "server.log")
	logf, err := os.Create(logPath)
	if err != nil {
		return State{}, err
	}
	defer logf.Close()

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"CLOUD_PROVIDER=local",
		"LOCAL_PATH="+filepath.Join(data, "store"),
		"GRAPH_DATA_CACHE_DIR="+filepath.Join(data, "cache"),
		"GRAPH_NAMESPACE=default", "GRAPH_ID=default", "GRAPH_CELL_ID=cell-0", "GRAPH_CELLS=cell-0", "GRAPH_NODE_ID=node-0",
		"GRAPH_HTTP_ADDR="+st.HTTP, "GRAPH_ADMIN_ADDR="+st.Admin, "GRAPH_BOLT_ADDR="+st.Bolt,
		"GRAPH_BOLT_NODE_ADDRESSES=node-0="+st.Bolt, "GRAPH_ADVERTISED_BOLT_ADDR="+st.Bolt,
		"GRAPH_AUTH_TOKEN_FILE="+filepath.Join(home, "token"),
		// It only listens on 127.0.0.1, and every request needs the token.
		"GRAPH_ALLOW_PLAINTEXT=true",
		// graph-node's query futures need more than the default stack.
		"RUST_MIN_STACK=33554432",
	)
	if os.Getenv("RUST_LOG") == "" {
		cmd.Env = append(cmd.Env, "RUST_LOG=warn")
	}
	cmd.Stdout, cmd.Stderr = logf, logf
	// Its own session, so it keeps running after this command exits and
	// doesn't get the terminal's Ctrl-C.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return State{}, fmt.Errorf("starting HydraDB: %w", err)
	}
	st.PID = cmd.Process.Pid
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.After(60 * time.Second)
	for !ready(st.Admin) {
		select {
		case err := <-exited:
			return State{}, fmt.Errorf("HydraDB exited while starting (%v). Last lines of %s:\n%s", err, logPath, tail(logPath, 15))
		case <-deadline:
			cmd.Process.Kill()
			return State{}, fmt.Errorf("HydraDB did not become ready within 60s. Last lines of %s:\n%s", logPath, tail(logPath, 15))
		case <-ctx.Done():
			cmd.Process.Kill()
			return State{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	b, _ := json.Marshal(st)
	return st, os.WriteFile(filepath.Join(home, "server.json"), b, 0o600)
}

// Stop shuts the local server down, if it is running.
func Stop(home string) (bool, error) {
	path := filepath.Join(home, "server.json")
	st, _ := Running(home)
	if st.PID == 0 || syscall.Kill(st.PID, 0) != nil {
		os.Remove(path)
		return false, nil
	}
	if err := syscall.Kill(st.PID, syscall.SIGTERM); err != nil {
		return false, err
	}
	for range 150 {
		if syscall.Kill(st.PID, 0) != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if syscall.Kill(st.PID, 0) == nil {
		syscall.Kill(st.PID, syscall.SIGKILL)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return true, err
	}
	return true, nil
}

func freePorts(n int) ([]string, error) {
	var out []string
	for range n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		defer l.Close()
		out = append(out, l.Addr().String())
	}
	return out, nil
}

func tail(path string, n int) string {
	b, _ := os.ReadFile(path)
	lines := strings.Split(string(bytes.TrimSpace(b)), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}
