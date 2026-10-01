package osv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
