package hydra

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryFollowsCursorAndPinsEpoch(t *testing.T) {
	var calls []request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/graphs/default/query" || r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("X-Graph-Namespace") != "default" {
			t.Errorf("bad request %s %v", r.URL.Path, r.Header)
		}
		var req request
		json.NewDecoder(r.Body).Decode(&req)
		calls = append(calls, req)
		if req.Cursor == nil {
			w.Write([]byte(`{"columns":["id"],"rows":[[{"type":"vertex_id","value":1}]],"read_epoch":9,"next_cursor":1}`))
			return
		}
		w.Write([]byte(`{"columns":["id"],"rows":[[{"type":"vertex_id","value":2}]],"read_epoch":9,"next_cursor":null}`))
	}))
	defer srv.Close()

	res, err := New(srv.URL, "tok").Query(context.Background(), "MATCH (n {id: $x}) RETURN n.id", map[string]any{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 || len(calls) != 2 {
		t.Fatalf("rows %d calls %d", len(res.Rows), len(calls))
	}
	if n, _ := res.Rows[1][0].Int(); n != 2 {
		t.Fatalf("second row %d", n)
	}
	if calls[1].ReadEpoch == nil || *calls[1].ReadEpoch != 9 || calls[0].Parameters["x"] != float64(1) {
		t.Fatalf("second call %+v", calls[1])
	}
}

func TestQueryReturnsTypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"invalid_query","message":"RETURN * is not supported"}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, "tok").Query(context.Background(), "MATCH (n) RETURN *", nil)
	var he *Error
	if !errors.As(err, &he) || he.Code != "invalid_query" || he.Status != 400 {
		t.Fatalf("err %v", err)
	}
}

func TestValueDecoding(t *testing.T) {
	s, err := Value{Type: "string", Value: json.RawMessage(`"lodash"`)}.String()
	if err != nil || s != "lodash" {
		t.Fatal(s, err)
	}
	if s, err := (Value{Type: "null"}).String(); err != nil || s != "" {
		t.Fatal(s, err)
	}
	if _, err := (Value{Type: "string", Value: json.RawMessage(`"x"`)}).Int(); err == nil {
		t.Fatal("string read as int")
	}
}

func TestValuePath(t *testing.T) {
	// Trimmed from a real algo.SPpaths answer.
	raw := `{"nodes":[{"id":100,"labels":["Project"],"properties":{"path":{"String":"/x/app"}}},
		{"id":101,"labels":["Pkg"],"properties":{"name":{"String":"a"},"project":{"Integer":100}}}],
		"relationships":[{"id":2,"edge_type":"DEPENDS_ON","src":100,"dst":101,"properties":{}}]}`
	nodes, err := Value{Type: "path", Value: json.RawMessage(raw)}.Path()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Props["path"] != "/x/app" || nodes[1].Props["project"] != int64(100) || nodes[1].Labels[0] != "Pkg" {
		t.Fatalf("nodes %+v", nodes)
	}
}
