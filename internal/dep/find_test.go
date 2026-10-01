package dep

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFindSkipsInstalledCopies(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"web/package-lock.json",
		"web/node_modules/left-pad/package-lock.json",
		"api/Cargo.lock",
		"api/target/debug/Cargo.lock",
		".cache/uv.lock",
		"tools/go.mod",
		"tools/README.md",
	} {
		p = filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, nil, 0o644)
	}
	got, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		got[i], _ = filepath.Rel(root, got[i])
	}
	if !slices.Equal(got, []string{"api/Cargo.lock", "tools/go.mod", "web/package-lock.json"}) {
		t.Fatalf("found %v", got)
	}
}

func TestReadUsesManifest(t *testing.T) {
	dir := t.TempDir()
	copyFile := func(src, dst string) {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, dst), b, 0o644)
	}
	copyFile("testdata/yarn.lock", "yarn.lock")
	copyFile("testdata/yarn-package.json", "package.json")
	g, err := Read(t.Context(), filepath.Join(dir, "yarn.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(g.Direct, npm("express", "4.18.2")) {
		t.Fatalf("direct %v", g.Direct)
	}
}
