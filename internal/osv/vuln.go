package osv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
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

// FixedIn returns the oldest listed fixed version, newer than
// p.Version, that no range of the record still covers, or "" when there
// is none. Git commit ranges are skipped, since they name commits rather
// than versions.
func (v *Vuln) FixedIn(p dep.Pkg) string {
	type interval struct {
		from, to  string
		inclusive bool // to is a last_affected version
	}
	var spans []interval
	var fixes []string
	for _, a := range v.Affected {
		if a.Package.Ecosystem != p.Eco || !samePkg(p.Eco, a.Package.Name, p.Name) {
			continue
		}
		for _, r := range a.Ranges {
			if r.Type == "GIT" {
				continue
			}
			open := ""
			for _, e := range r.Events {
				switch {
				case e["introduced"] != "":
					open = e["introduced"]
				case e["fixed"] != "":
					spans = append(spans, interval{open, e["fixed"], false})
					fixes = append(fixes, e["fixed"])
					open = ""
				case e["last_affected"] != "":
					spans = append(spans, interval{open, e["last_affected"], true})
					open = ""
				}
			}
			if open != "" {
				spans = append(spans, interval{open, "", false})
			}
		}
	}
	affected := func(ver string) bool {
		for _, s := range spans {
			if s.from != "0" && version.Compare(ver, s.from) < 0 {
				continue
			}
			c := version.Compare(ver, s.to)
			if s.to == "" || c < 0 || s.inclusive && c == 0 {
				return true
			}
		}
		return false
	}
	slices.SortFunc(fixes, version.Compare)
	for _, f := range fixes {
		if version.Compare(f, p.Version) > 0 && !affected(f) {
			return f
		}
	}
	return ""
}

func samePkg(eco, a, b string) bool {
	if eco == dep.PyPI {
		return dep.NormalizePy(a) == dep.NormalizePy(b)
	}
	return a == b
}
