package dep

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// lockfiles maps each lockfile name depblast reads to its parser.
var lockfiles = map[string]func(ctx context.Context, path string) (*Graph, error){
	"package-lock.json":   withFile(ParseNpm),
	"npm-shrinkwrap.json": withFile(ParseNpm),
	"pnpm-lock.yaml":      withFile(ParsePnpm),
	"Cargo.lock":          withFile(ParseCargo),
	"uv.lock":             withFile(ParseUv),
	"yarn.lock":           withManifest(ParseYarn, "package.json"),
	"poetry.lock":         withManifest(ParsePoetry, "pyproject.toml"),
	"go.mod":              GoGraph,
}

// skipDirs are never searched: installed copies of dependencies, build
// output and virtualenvs hold lockfiles that aren't the project's.
var skipDirs = map[string]bool{"node_modules": true, "vendor": true, "target": true, "venv": true, "__pycache__": true}

// Find lists the lockfiles under root, sorted. Hidden folders are skipped.
func Find(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || skipDirs[d.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		if lockfiles[d.Name()] != nil {
			out = append(out, path)
		}
		return nil
	})
	slices.Sort(out)
	return out, err
}

// Read parses one lockfile found by Find.
func Read(ctx context.Context, path string) (*Graph, error) {
	return lockfiles[filepath.Base(path)](ctx, path)
}

func withFile(parse func(string, io.Reader) (*Graph, error)) func(context.Context, string) (*Graph, error) {
	return func(_ context.Context, path string) (*Graph, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return parse(path, f)
	}
}

// withManifest also opens the manifest next to the lockfile, if there is
// one, for formats that need it to know the project's own dependencies.
func withManifest(parse func(string, io.Reader, io.Reader) (*Graph, error), manifest string) func(context.Context, string) (*Graph, error) {
	return func(_ context.Context, path string) (*Graph, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		var m io.Reader
		if mf, err := os.Open(filepath.Join(filepath.Dir(path), manifest)); err == nil {
			defer mf.Close()
			m = mf
		}
		return parse(path, f, m)
	}
}
