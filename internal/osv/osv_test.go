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
