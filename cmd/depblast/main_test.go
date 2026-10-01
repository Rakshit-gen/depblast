package main

import "testing"

func TestSplitSpec(t *testing.T) {
	for in, want := range map[string][2]string{
		"lodash@4.17.20":           {"lodash", "4.17.20"},
		"lodash":                   {"lodash", ""},
		"@babel/core":              {"@babel/core", ""},
		"@babel/core@<7.23.2":      {"@babel/core", "<7.23.2"},
		"golang.org/x/net@v0.17.0": {"golang.org/x/net", "v0.17.0"},
	} {
		if n, v := splitSpec(in); n != want[0] || v != want[1] {
			t.Errorf("%s: %q %q", in, n, v)
		}
	}
}

func TestUnder(t *testing.T) {
	roots := []string{"/code/web"}
	for path, want := range map[string]bool{
		"/code/web":                       true,
		"/code/web/package-lock.json":     true,
		"/code/website/package-lock.json": false,
		"/code/api/Cargo.lock":            false,
	} {
		if under(path, roots) != want {
			t.Errorf("%s: want %v", path, want)
		}
	}
}
