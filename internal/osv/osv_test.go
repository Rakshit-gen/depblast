package osv

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rakshit-gen/depblast/internal/dep"
)

func TestAffectingFollowsPages(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct{ Queries []query }
		json.NewDecoder(r.Body).Decode(&body)
		switch {
		case calls == 1 && len(body.Queries) == 2:
			w.Write([]byte(`{"results":[{"vulns":[{"id":"A"}],"next_page_token":"p2"},{}]}`))
		case calls == 2 && len(body.Queries) == 1 && body.Queries[0].PageToken == "p2" && body.Queries[0].Package.Name == "lodash":
			w.Write([]byte(`{"results":[{"vulns":[{"id":"B"}]}]}`))
		default:
			t.Errorf("unexpected call %d: %+v", calls, body)
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	got, err := c.Affecting(t.Context(), []dep.Pkg{{Eco: dep.Npm, Name: "lodash", Version: "4.17.20"}, {Eco: dep.Npm, Name: "left-pad", Version: "1.3.0"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0]) != 2 || got[0][1].ID != "B" || len(got[1]) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestVulnCache(t *testing.T) {
	fetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches++
		if r.URL.Path != "/v1/vulns/GHSA-35jh-r3h4-6jhm" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Write([]byte(`{"id":"GHSA-35jh-r3h4-6jhm","modified":"2026-09-10","summary":"Command Injection in lodash","database_specific":{"severity":"HIGH"}}`))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	cache := t.TempDir()
	ref := Ref{ID: "GHSA-35jh-r3h4-6jhm", Modified: "2026-09-10"}
	for range 2 {
		v, err := c.Vuln(t.Context(), ref, cache)
		if err != nil {
			t.Fatal(err)
		}
		if v.Summary != "Command Injection in lodash" || v.DatabaseSpecific.Severity != "HIGH" {
			t.Fatalf("vuln %+v", v)
		}
	}
	if fetches != 1 {
		t.Fatalf("fetched %d times, want 1", fetches)
	}
	// A newer modified time means the cached copy is stale.
	ref.Modified = "2026-10-01"
	c.Vuln(t.Context(), ref, cache)
	if fetches != 2 {
		t.Fatalf("stale record not refetched")
	}
}
