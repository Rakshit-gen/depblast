// Package osv asks OSV.dev which package versions have known
// vulnerabilities. OSV.dev is free and needs no key. Only package names,
// ecosystems and versions are sent, never file paths or project names.
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Rakshit-gen/depblast/internal/dep"
)

// Client talks to the OSV.dev API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New() *Client {
	return &Client{BaseURL: "https://api.osv.dev", HTTP: &http.Client{Timeout: time.Minute}}
}

// Ref is a vulnerability id with the time it last changed.
type Ref struct {
	ID       string `json:"id"`
	Modified string `json:"modified"`
}

// batchSize is the most queries OSV.dev takes in one querybatch call.
const batchSize = 1000

type query struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Version   string `json:"version"`
	PageToken string `json:"page_token,omitempty"`
}

// Affecting returns, for each package, the vulnerabilities that affect
// that exact version. Batches of 1000 go out four at a time; one batch
// takes about 2 seconds.
func (c *Client) Affecting(ctx context.Context, pkgs []dep.Pkg) ([][]Ref, error) {
	out := make([][]Ref, len(pkgs))
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
		sem  = make(chan struct{}, 4)
	)
	for start := 0; start < len(pkgs); start += batchSize {
		end := min(start+batchSize, len(pkgs))
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := c.batch(ctx, pkgs[start:end], out[start:end]); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		return nil, errs[0]
	}
	return out, nil
}

// batch asks about up to batchSize packages and fills out, which is as
// long as pkgs.
func (c *Client) batch(ctx context.Context, pkgs []dep.Pkg, out [][]Ref) error {
	qs := make([]query, len(pkgs))
	idx := make([]int, len(pkgs))
	for i, p := range pkgs {
		qs[i].Package.Name, qs[i].Package.Ecosystem, qs[i].Version = p.Name, p.Eco, p.Version
		idx[i] = i
	}
	// A package with very many vulnerabilities is paged; ask again for
	// just those until every page is read.
	for len(qs) > 0 {
		var resp struct {
			Results []struct {
				Vulns         []Ref  `json:"vulns"`
				NextPageToken string `json:"next_page_token"`
			} `json:"results"`
		}
		if err := c.post(ctx, "/v1/querybatch", map[string]any{"queries": qs}, &resp); err != nil {
			return err
		}
		if len(resp.Results) != len(qs) {
			return fmt.Errorf("osv.dev: asked %d queries, got %d answers", len(qs), len(resp.Results))
		}
		var more []query
		var moreIdx []int
		for i, r := range resp.Results {
			out[idx[i]] = append(out[idx[i]], r.Vulns...)
			if r.NextPageToken != "" {
				q := qs[i]
				q.PageToken = r.NextPageToken
				more = append(more, q)
				moreIdx = append(moreIdx, idx[i])
			}
		}
		qs, idx = more, moreIdx
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("osv.dev: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("osv.dev: %s %s: %s", req.Method, req.URL.Path, resp.Status)
	}
	return json.Unmarshal(data, out)
}
