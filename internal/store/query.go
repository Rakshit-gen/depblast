package store

import (
	"context"
	"fmt"
	"time"

	"github.com/Rakshit-gen/depblast/internal/dep"
	"github.com/Rakshit-gen/depblast/internal/hydra"
)

// Project is one indexed lockfile.
type Project struct {
	ID       uint64
	Path     string
	Indexed  time.Time
	Packages int
}

// Projects lists every indexed project.
func Projects(ctx context.Context, c *hydra.Client) ([]Project, error) {
	res, err := c.Query(ctx, `MATCH (p:Project) RETURN p.id AS id, p.path AS path, p.indexed AS indexed, p.packages AS packages ORDER BY path`, nil)
	if err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(res.Rows))
	for _, r := range res.Rows {
		var p Project
		id, err1 := r[0].Int()
		path, err2 := r[1].String()
		indexed, err3 := r[2].Int()
		n, err4 := r[3].Int()
		if err := firstErr(err1, err2, err3, err4); err != nil {
			return nil, err
		}
		p.ID, p.Path, p.Indexed, p.Packages = uint64(id), path, time.Unix(indexed, 0), int(n)
		out = append(out, p)
	}
	return out, nil
}

// Copy is one package version as locked by one project.
type Copy struct {
	ID      uint64
	Project uint64
	Pkg     dep.Pkg
}

// Copies finds every locked copy of the package called name, in every
// project and at every version.
func Copies(ctx context.Context, c *hydra.Client, name string) ([]Copy, error) {
	res, err := c.Query(ctx, `MATCH (v:Pkg) WHERE v.name = $name RETURN v.id AS id, v.project AS project, v.eco AS eco, v.version AS version`, map[string]any{"name": name})
	if err != nil {
		return nil, err
	}
	out := make([]Copy, 0, len(res.Rows))
	for _, r := range res.Rows {
		id, err1 := r[0].Int()
		proj, err2 := r[1].Int()
		eco, err3 := r[2].String()
		version, err4 := r[3].String()
		if err := firstErr(err1, err2, err3, err4); err != nil {
			return nil, err
		}
		out = append(out, Copy{ID: uint64(id), Project: uint64(proj), Pkg: dep.Pkg{Eco: eco, Name: name, Version: version}})
	}
	return out, nil
}

// MaxDepth bounds path searches. 16 is the most HydraDB v0.2 allows
// (max_traversal_hops, not configurable). A copy deeper than this shows
// up in Copies but gets no path.
const MaxDepth = 16

// Paths returns up to n shortest dependency chains from a project to one
// of its package copies, each listed from the project's direct dependency
// down to the copy.
func Paths(ctx context.Context, c *hydra.Client, project, copyID uint64, n int) ([][]dep.Pkg, error) {
	res, err := c.Query(ctx, `CALL algo.SPpaths({sourceNode: $from, targetNode: $to, relTypes: ['DEPENDS_ON'], maxLen: $max, pathCount: $n}) YIELD path RETURN path`,
		map[string]any{"from": project, "to": copyID, "max": MaxDepth, "n": n})
	if err != nil {
		return nil, err
	}
	var out [][]dep.Pkg
	for _, r := range res.Rows {
		nodes, err := r[0].Path()
		if err != nil {
			return nil, err
		}
		var chain []dep.Pkg
		for _, nd := range nodes[1:] { // nodes[0] is the project
			eco, _ := nd.Props["eco"].(string)
			name, _ := nd.Props["name"].(string)
			version, _ := nd.Props["version"].(string)
			chain = append(chain, dep.Pkg{Eco: eco, Name: name, Version: version})
		}
		out = append(out, chain)
	}
	return out, nil
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return fmt.Errorf("unexpected result from HydraDB: %w", err)
		}
	}
	return nil
}

// Distinct lists every package version locked by at least one project,
// once each.
func Distinct(ctx context.Context, c *hydra.Client) ([]dep.Pkg, error) {
	res, err := c.Query(ctx, `MATCH (v:Pkg) RETURN DISTINCT v.eco AS eco, v.name AS name, v.version AS version`, nil)
	if err != nil {
		return nil, err
	}
	out := make([]dep.Pkg, 0, len(res.Rows))
	for _, r := range res.Rows {
		eco, err1 := r[0].String()
		name, err2 := r[1].String()
		version, err3 := r[2].String()
		if err := firstErr(err1, err2, err3); err != nil {
			return nil, err
		}
		out = append(out, dep.Pkg{Eco: eco, Name: name, Version: version})
	}
	return out, nil
}
