package osv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rakshit-gen/depblast/internal/dep"
	"github.com/Rakshit-gen/depblast/internal/version"
)

// Vuln is the part of an OSV record depblast shows.
type Vuln struct {
	ID       string   `json:"id"`
	Modified string   `json:"modified"`
	Summary  string   `json:"summary"`
	Aliases  []string `json:"aliases"`
	Affected []struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		Ranges []struct {
			Type   string              `json:"type"`
			Events []map[string]string `json:"events"`
		} `json:"ranges"`
	} `json:"affected"`
	DatabaseSpecific struct {
		Severity string `json:"severity"`
	} `json:"database_specific"`
}

// Vuln fetches one record. With a cache folder, a record is read from
// disk when it is still the version ref names, and saved after fetching.
func (c *Client) Vuln(ctx context.Context, ref Ref, cache string) (*Vuln, error) {
	path := ""
	if cache != "" && !strings.ContainsAny(ref.ID, `/\`) {
		path = filepath.Join(cache, ref.ID+".json")
		if b, err := os.ReadFile(path); err == nil {
			var v Vuln
			if json.Unmarshal(b, &v) == nil && v.Modified == ref.Modified {
				return &v, nil
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/v1/vulns/"+url.PathEscape(ref.ID), nil)
	if err != nil {
		return nil, err
	}
	var v Vuln
	if err := c.do(req, &v); err != nil {
		return nil, err
	}
	if path != "" {
		if b, err := json.Marshal(v); err == nil && os.MkdirAll(cache, 0o700) == nil {
			os.WriteFile(path, b, 0o600) // a failed cache write only costs a refetch
		}
	}
	return &v, nil
}

// FixedIn returns the oldest version newer than p.Version that the record
// says fixes p's package, or "" when no fix is listed. Git commit ranges
// are skipped, since they name commits rather than versions.
func (v *Vuln) FixedIn(p dep.Pkg) string {
	best := ""
	for _, a := range v.Affected {
		if a.Package.Ecosystem != p.Eco || !samePkg(p.Eco, a.Package.Name, p.Name) {
			continue
		}
		for _, r := range a.Ranges {
			if r.Type == "GIT" {
				continue
			}
			for _, e := range r.Events {
				f := e["fixed"]
				if f != "" && version.Compare(f, p.Version) > 0 && (best == "" || version.Compare(f, best) < 0) {
					best = f
				}
			}
		}
	}
	return best
}

func samePkg(eco, a, b string) bool {
	if eco == dep.PyPI {
		return dep.NormalizePy(a) == dep.NormalizePy(b)
	}
	return a == b
}
