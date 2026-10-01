package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Rakshit-gen/depblast/internal/dep"
)

type pkgJSON struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}

func toJSON(p dep.Pkg) pkgJSON { return pkgJSON{p.Eco, p.Name, p.Version} }

func chainJSON(chain []dep.Pkg) []pkgJSON {
	out := make([]pkgJSON, len(chain))
	for i, p := range chain {
		out[i] = toJSON(p)
	}
	return out
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func formatChainJSON(chain []pkgJSON) string {
	parts := make([]string, len(chain))
	for i, p := range chain {
		parts[i] = p.Name + " " + p.Version
	}
	return strings.Join(parts, " > ")
}
