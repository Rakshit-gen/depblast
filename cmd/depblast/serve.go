package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Rakshit-gen/depblast/internal/server"
	"github.com/Rakshit-gen/depblast/internal/store"
)

const serverUsage = `usage: depblast server status|stop|log|rebuild

depblast starts its own HydraDB in the background the first time a command
needs it, and leaves it running so later commands are quick.

  status  say whether it is running, where, and how much data it holds
  stop    shut it down (the next command starts it again)
  log     print its log file
  rebuild delete the data folder and index every known lockfile again

HydraDB v0.2 does not reclaim space from replaced data, so the data folder
grows each time a changed lockfile is indexed. rebuild shrinks it back.
Lockfiles that no longer exist are dropped.
`

func runServer(ctx context.Context, args []string) error {
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, serverUsage)
		os.Exit(2)
	}
	if os.Getenv("DEPBLAST_HYDRA_URL") != "" {
		return fmt.Errorf("DEPBLAST_HYDRA_URL is set, so depblast uses that server and doesn't run its own")
	}
	home, err := server.Home()
	if err != nil {
		return err
	}
	switch args[0] {
	case "status":
		st, ok := server.Running(home)
		if !ok {
			fmt.Println("not running")
		} else {
			fmt.Printf("running, pid %d, http %s\n", st.PID, st.HTTP)
		}
		fmt.Printf("data: %s (%s)\n", filepath.Join(home, "data"), humanSize(dirSize(filepath.Join(home, "data"))))
	case "stop":
		stopped, err := server.Stop(home)
		if err != nil {
			return err
		}
		if stopped {
			fmt.Println("stopped")
		} else {
			fmt.Println("was not running")
		}
	case "log":
		b, err := os.ReadFile(filepath.Join(home, "server.log"))
		if err != nil {
			return err
		}
		os.Stdout.Write(b)
	case "rebuild":
		return rebuild(ctx, home)
	default:
		fmt.Fprint(os.Stderr, serverUsage)
		os.Exit(2)
	}
	return nil
}

func rebuild(ctx context.Context, home string) error {
	c, err := server.Connect(ctx)
	if err != nil {
		return err
	}
	projects, err := store.Projects(ctx, c)
	if err != nil {
		return err
	}
	var paths []string
	for _, p := range projects {
		if _, err := os.Stat(p.Path); err != nil {
			fmt.Printf("dropping %s: %v\n", rel(p.Path), err)
			continue
		}
		paths = append(paths, p.Path)
	}
	data := filepath.Join(home, "data")
	before := dirSize(data)
	if _, err := server.Stop(home); err != nil {
		return err
	}
	if err := os.RemoveAll(data); err != nil {
		return err
	}
	if len(paths) > 0 {
		if err := runIndex(ctx, paths); err != nil {
			return err
		}
	}
	fmt.Printf("data folder: %s before, %s now\n", humanSize(before), humanSize(dirSize(data)))
	return nil
}

func dirSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}
