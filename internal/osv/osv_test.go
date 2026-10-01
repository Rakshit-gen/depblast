package osv

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
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

func TestAffectingSplitsBatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Queries []query }
		json.NewDecoder(r.Body).Decode(&body)
		if len(body.Queries) > batchSize {
			t.Errorf("batch of %d", len(body.Queries))
		}
		// Answer each query with its own version as the vulnerability
		// id, so a result put in the wrong slot shows up.
		var resp struct {
			Results []struct {
				Vulns []Ref `json:"vulns"`
			} `json:"results"`
		}
		resp.Results = make([]struct {
			Vulns []Ref `json:"vulns"`
		}, len(body.Queries))
		for i, q := range body.Queries {
			resp.Results[i].Vulns = []Ref{{ID: q.Version}}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	pkgs := make([]dep.Pkg, 2*batchSize+7)
	for i := range pkgs {
		pkgs[i] = dep.Pkg{Eco: dep.Npm, Name: "p", Version: strconv.Itoa(i)}
	}
	got, err := c.Affecting(t.Context(), pkgs)
	if err != nil {
		t.Fatal(err)
	}
	for i, refs := range got {
		if len(refs) != 1 || refs[0].ID != strconv.Itoa(i) {
			t.Fatalf("package %d got %+v", i, refs)
		}
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

func TestFixedIn(t *testing.T) {
	var v Vuln
	json.Unmarshal([]byte(`{"affected":[
		{"package":{"name":"lodash","ecosystem":"npm"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":"4.17.21"}]}]},
		{"package":{"name":"lodash","ecosystem":"npm"},"ranges":[{"type":"SEMVER","events":[{"introduced":"3.0.0"},{"fixed":"3.10.2"}]}]},
		{"package":{"name":"lodash","ecosystem":"npm"},"ranges":[{"type":"GIT","events":[{"introduced":"0"},{"fixed":"abc123"}]}]},
		{"package":{"name":"Django","ecosystem":"PyPI"},"ranges":[{"type":"ECOSYSTEM","events":[{"introduced":"4.2"},{"fixed":"4.2.7"},{"introduced":"5.0"},{"fixed":"5.0.1"}]}]},
		{"package":{"name":"left-pad","ecosystem":"npm"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"last_affected":"1.3.0"}]}]}
	]}`), &v)
	for _, c := range []struct {
		p    dep.Pkg
		want string
	}{
		{dep.Pkg{Eco: dep.Npm, Name: "lodash", Version: "4.17.20"}, "4.17.21"},
		// 3.10.2 closes the second range, but the first still covers it.
		{dep.Pkg{Eco: dep.Npm, Name: "lodash", Version: "3.5.0"}, "4.17.21"},
		{dep.Pkg{Eco: dep.Npm, Name: "lodash", Version: "4.17.21"}, ""},
		{dep.Pkg{Eco: dep.PyPI, Name: "django", Version: "4.2.1"}, "4.2.7"},
		{dep.Pkg{Eco: dep.PyPI, Name: "django", Version: "5.0"}, "5.0.1"},
		{dep.Pkg{Eco: dep.Npm, Name: "left-pad", Version: "1.2.0"}, ""},
		{dep.Pkg{Eco: dep.Cargo, Name: "lodash", Version: "1.0.0"}, ""},
	} {
		if got := v.FixedIn(c.p); got != c.want {
			t.Errorf("%v: %q, want %q", c.p, got, c.want)
		}
	}
}

func TestDedupe(t *testing.T) {
	v := func(id, sev string, aliases ...string) *Vuln {
		x := &Vuln{ID: id, Aliases: aliases}
		x.DatabaseSpecific.Severity = sev
		return x
	}
	// Two GHSA ids for one lodash issue, linked through a shared CVE, and
	// an unrelated nanoid issue.
	got := Dedupe([]*Vuln{
		v("GHSA-r5fr-rjxr-66jc", "", "CVE-2026-4800"),
		v("GHSA-35jh-r3h4-6jhm", "HIGH", "CVE-2021-23337", "CVE-2026-4800"),
		v("GHSA-28wg-ghj8-5hjv", "HIGH"),
	})
	if len(got) != 2 {
		t.Fatalf("kept %d", len(got))
	}
	lodash := got[1]
	if lodash.ID != "GHSA-35jh-r3h4-6jhm" || !slices.Contains(lodash.Aliases, "GHSA-r5fr-rjxr-66jc") {
		t.Fatalf("lodash kept %s with aliases %v", lodash.ID, lodash.Aliases)
	}
}
