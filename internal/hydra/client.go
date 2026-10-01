// Package hydra is a small client for HydraDB's HTTP query API.
package hydra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client sends Cypher statements to one graph cell.
type Client struct {
	URL       string // for example http://127.0.0.1:8443
	Token     string
	Namespace string
	Graph     string
	Cell      string
	HTTP      *http.Client
}

// New returns a client for the single-cell setup depblast runs.
func New(url, token string) *Client {
	return &Client{
		URL:       strings.TrimRight(url, "/"),
		Token:     token,
		Namespace: "default",
		Graph:     "default",
		Cell:      "cell-0",
		HTTP:      &http.Client{Timeout: 5 * time.Minute},
	}
}

// Value is one cell of a result row, as HydraDB tags it: vertex_id,
// integer, signed_integer, float, boolean, string, list, path or null.
type Value struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// Result holds every row of a query, all pages joined.
type Result struct {
	Columns []string
	Rows    [][]Value
}

type request struct {
	CellID     string         `json:"cell_id"`
	Query      string         `json:"query"`
	Parameters map[string]any `json:"parameters,omitempty"`
	ReadEpoch  *uint64        `json:"read_epoch,omitempty"`
	PageSize   int            `json:"page_size,omitempty"`
	Cursor     *uint64        `json:"cursor,omitempty"`
}

type response struct {
	Columns    []string  `json:"columns"`
	Rows       [][]Value `json:"rows"`
	ReadEpoch  *uint64   `json:"read_epoch"`
	NextCursor *uint64   `json:"next_cursor"`
}

// Query runs one statement and follows the result cursor until every row
// is read. Later pages pin the first page's read epoch, so all rows come
// from the same snapshot.
func (c *Client) Query(ctx context.Context, query string, params map[string]any) (*Result, error) {
	req := request{CellID: c.Cell, Query: query, Parameters: params}
	res := &Result{}
	for {
		var page response
		if err := c.post(ctx, req, &page); err != nil {
			return nil, err
		}
		if res.Columns == nil {
			res.Columns = page.Columns
		}
		res.Rows = append(res.Rows, page.Rows...)
		if page.NextCursor == nil {
			return res, nil
		}
		req.Cursor = page.NextCursor
		req.ReadEpoch = page.ReadEpoch
	}
}

func (c *Client) post(ctx context.Context, body request, out *response) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	r, err := http.NewRequestWithContext(ctx, "POST", c.URL+"/v1/graphs/"+c.Graph+"/query", bytes.NewReader(b))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+c.Token)
	r.Header.Set("X-Graph-Namespace", c.Namespace)
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hydradb: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return json.Unmarshal(data, out)
}
