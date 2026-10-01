package main

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Rakshit-gen/depblast/internal/dep"
	"github.com/Rakshit-gen/depblast/internal/osv"
	"github.com/Rakshit-gen/depblast/internal/server"
	"github.com/Rakshit-gen/depblast/internal/store"
	"github.com/Rakshit-gen/depblast/internal/version"
)

var severityRank = map[string]int{"CRITICAL": 0, "HIGH": 1, "MODERATE": 2, "MEDIUM": 2, "LOW": 3}

func runAudit(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "print the results as JSON")
	minSev := fs.String("min", "", "only show `severity` and worse: critical, high, moderate or low")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: depblast audit [flags]\n\n"+
			"Checks every package version in the indexed projects against OSV.dev and lists\n"+
			"the known vulnerabilities, worst first, with the projects that ship each one\n"+
			"and the chain that pulls it in. Only package names, ecosystems and versions are\n"+
			"sent to OSV.dev. Advisories without a severity are shown unless -min is set.\n\n"+
			"Exits with status 3 when it lists any vulnerability, so it can fail a CI job.\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	limit := len(severityRank)
	if *minSev != "" {
		r, ok := severityRank[strings.ToUpper(*minSev)]
		if !ok {
			return fmt.Errorf("-min %q: use critical, high, moderate or low", *minSev)
		}
		limit = r
	}
	c, err := server.Connect(ctx)
	if err != nil {
		return err
	}
	pkgs, err := store.Distinct(ctx, c)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		fmt.Println("nothing indexed yet, run: depblast index PATH")
		return nil
	}
	if !*asJSON {
		fmt.Fprintf(os.Stderr, "checking %d package versions against OSV.dev...\n", len(pkgs))
	}
	o := osv.New()
	refs, err := o.Affecting(ctx, pkgs)
	if err != nil {
		return err
	}
	byVuln := map[string][]dep.Pkg{}
	unique := map[string]osv.Ref{}
	for i, rs := range refs {
		for _, r := range rs {
			byVuln[r.ID] = append(byVuln[r.ID], pkgs[i])
			unique[r.ID] = r
		}
	}
	vulns, err := fetchVulns(ctx, o, unique)
	if err != nil {
		return err
	}
	// Merged records keep the affected packages of every id they absorbed.
	vulns = osv.Dedupe(vulns)
	vulns = slices.DeleteFunc(vulns, func(v *osv.Vuln) bool { return rank(v) > limit })
	for _, v := range vulns {
		for _, a := range v.Aliases {
			for _, p := range byVuln[a] {
				if !slices.Contains(byVuln[v.ID], p) {
					byVuln[v.ID] = append(byVuln[v.ID], p)
				}
			}
		}
	}
	slices.SortFunc(vulns, func(a, b *osv.Vuln) int {
		if d := rank(a) - rank(b); d != 0 {
			return d
		}
		return strings.Compare(a.ID, b.ID)
	})

	projects, err := store.Projects(ctx, c)
	if err != nil {
		return err
	}
	pathOf := map[uint64]string{}
	for _, p := range projects {
		pathOf[p.ID] = p.Path
	}
	type shipJSON struct {
		Lockfile string    `json:"lockfile"`
		Path     []pkgJSON `json:"path"` // empty when not reachable
	}
	type affectedJSON struct {
		Package pkgJSON    `json:"package"`
		FixedIn string     `json:"fixed_in,omitempty"`
		Shipped []shipJSON `json:"shipped_by"`
	}
	type vulnJSON struct {
		ID       string         `json:"id"`
		Aliases  []string       `json:"aliases,omitempty"`
		Severity string         `json:"severity"`
		Summary  string         `json:"summary"`
		Affected []affectedJSON `json:"affected"`
	}
	out := []vulnJSON{}
	for _, v := range vulns {
		sev := v.DatabaseSpecific.Severity
		if sev == "" {
			sev = "UNRATED"
		}
		vj := vulnJSON{ID: v.ID, Aliases: v.Aliases, Severity: sev, Summary: v.Summary}
		affected := byVuln[v.ID]
		slices.SortFunc(affected, compareVersionsOf)
		for _, p := range affected {
			aj := affectedJSON{Package: toJSON(p), FixedIn: v.FixedIn(p), Shipped: []shipJSON{}}
			copies, err := store.Copies(ctx, c, p.Name)
			if err != nil {
				return err
			}
			for _, cp := range copies {
				if cp.Pkg != p {
					continue
				}
				chains, err := store.Paths(ctx, c, cp.Project, cp.ID, 1)
				if err != nil {
					return err
				}
				sj := shipJSON{Lockfile: pathOf[cp.Project], Path: []pkgJSON{}}
				if len(chains) > 0 {
					sj.Path = chainJSON(chains[0])
				}
				aj.Shipped = append(aj.Shipped, sj)
			}
			vj.Affected = append(vj.Affected, aj)
		}
		out = append(out, vj)
	}

	if *asJSON {
		if err := printJSON(out); err != nil {
			return err
		}
	} else {
		for _, v := range out {
			fmt.Printf("\n%s  %s  %s\n", v.Severity, v.ID, v.Summary)
			if len(v.Aliases) > 0 {
				fmt.Printf("  also known as %s\n", strings.Join(v.Aliases, ", "))
			}
			for _, a := range v.Affected {
				fix := "no fixed version listed"
				if a.FixedIn != "" {
					fix = "fixed in " + a.FixedIn
				}
				fmt.Printf("  %s %s, %s\n", a.Package.Name, a.Package.Version, fix)
				for _, sh := range a.Shipped {
					how := "not reachable from the project"
					if len(sh.Path) > 0 {
						how = formatChainJSON(sh.Path)
					}
					fmt.Printf("    %s: %s\n", rel(sh.Lockfile), how)
				}
			}
		}
		what := "known vulnerabilities"
		switch {
		case limit == 0:
			what = "critical vulnerabilities"
		case *minSev != "":
			what = strings.ToLower(*minSev) + " or worse vulnerabilities"
		}
		fmt.Printf("\n%d %s in %d indexed projects\n", len(vulns), what, len(projects))
	}
	if len(vulns) > 0 {
		return exitCode(3)
	}
	return nil
}

func rank(v *osv.Vuln) int {
	if r, ok := severityRank[v.DatabaseSpecific.Severity]; ok {
		return r
	}
	return len(severityRank)
}

func compareVersionsOf(a, b dep.Pkg) int {
	if c := strings.Compare(a.Name, b.Name); c != 0 {
		return c
	}
	return version.Compare(a.Version, b.Version)
}

// fetchVulns reads the full records, eight at a time, through the cache
// in depblast's home.
func fetchVulns(ctx context.Context, o *osv.Client, refs map[string]osv.Ref) ([]*osv.Vuln, error) {
	home, err := server.Home()
	if err != nil {
		return nil, err
	}
	cache := filepath.Join(home, "osv")
	var (
		mu       sync.Mutex
		out      []*osv.Vuln
		firstErr error
		wg       sync.WaitGroup
		sem      = make(chan struct{}, 8)
	)
	for _, r := range refs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			v, err := o.Vuln(ctx, r, cache)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				firstErr = cmp.Or(firstErr, err)
				return
			}
			out = append(out, v)
		}()
	}
	wg.Wait()
	return out, firstErr
}
