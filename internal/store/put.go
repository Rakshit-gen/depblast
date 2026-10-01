package store

import (
	"context"
	"path/filepath"
	"time"

	"github.com/Rakshit-gen/depblast/internal/dep"
	"github.com/Rakshit-gen/depblast/internal/hydra"
)

// batch is how many rows go into one UNWIND statement.
const batch = 1000

// Put replaces whatever is stored for the project read from lockfile (an
// absolute path) with g.
func Put(ctx context.Context, c *hydra.Client, lockfile string, g *dep.Graph) error {
	if err := Remove(ctx, c, lockfile); err != nil {
		return err
	}
	proj := ProjectID(lockfile)
	err := run(ctx, c, `UNWIND $rows AS row MERGE (n {id: row.id}) SET n:Project, n.path = row.path, n.kind = row.kind, n.indexed = row.indexed, n.packages = row.packages`,
		[]map[string]any{{"id": proj, "path": lockfile, "kind": filepath.Base(lockfile), "indexed": time.Now().Unix(), "packages": len(g.Deps)}})
	if err != nil {
		return err
	}

	var pkgs []map[string]any
	for _, p := range g.Packages() {
		pkgs = append(pkgs, map[string]any{"id": PkgID(proj, p), "eco": p.Eco, "name": p.Name, "version": p.Version, "project": proj})
	}
	if err := run(ctx, c, `UNWIND $rows AS row MERGE (n {id: row.id}) SET n:Pkg, n.eco = row.eco, n.name = row.name, n.version = row.version, n.project = row.project`, pkgs); err != nil {
		return err
	}

	// HydraDB wants exactly one label on each end of a batched edge, so
	// project edges and package edges go in separate statements.
	var direct, deps []map[string]any
	for _, p := range g.Direct {
		to := PkgID(proj, p)
		direct = append(direct, map[string]any{"s": proj, "d": to, "e": edgeID(proj, to)})
	}
	for _, p := range g.Packages() {
		from := PkgID(proj, p)
		for _, d := range g.Deps[p] {
			to := PkgID(proj, d)
			deps = append(deps, map[string]any{"s": from, "d": to, "e": edgeID(from, to)})
		}
	}
	if err := run(ctx, c, `UNWIND $rows AS row MATCH (s:Project {id: row.s}), (d:Pkg {id: row.d}) MERGE (s)-[r:DEPENDS_ON {id: row.e}]->(d)`, direct); err != nil {
		return err
	}
	return run(ctx, c, `UNWIND $rows AS row MATCH (s:Pkg {id: row.s}), (d:Pkg {id: row.d}) MERGE (s)-[r:DEPENDS_ON {id: row.e}]->(d)`, deps)
}

// Remove deletes a project and its packages. Removing a project that was
// never stored is not an error.
func Remove(ctx context.Context, c *hydra.Client, lockfile string) error {
	proj := ProjectID(lockfile)
	res, err := c.Query(ctx, `MATCH (v:Pkg) WHERE v.project = $p RETURN v.id AS id`, map[string]any{"p": proj})
	if err != nil {
		return err
	}
	rows := []map[string]any{{"id": proj}}
	for _, r := range res.Rows {
		id, err := r[0].Int()
		if err != nil {
			return err
		}
		rows = append(rows, map[string]any{"id": id})
	}
	return run(ctx, c, `UNWIND $rows AS row MATCH (n {id: row.id}) DETACH DELETE n`, rows)
}

func run(ctx context.Context, c *hydra.Client, stmt string, rows []map[string]any) error {
	for len(rows) > 0 {
		n := min(batch, len(rows))
		if _, err := c.Query(ctx, stmt, map[string]any{"rows": rows[:n]}); err != nil {
			return err
		}
		rows = rows[n:]
	}
	return nil
}
