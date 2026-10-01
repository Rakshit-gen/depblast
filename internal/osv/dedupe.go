package osv

import (
	"slices"
	"strings"
)

// Dedupe merges records that are the same issue. OSV often holds one
// issue under several ids (a GHSA record and a CVE or a second GHSA that
// alias each other), and a batch query returns all of them. Records that
// alias one another, directly or through a chain, become one: the record
// with a severity is kept, the lowest id if several have one, and the
// other ids are added to its aliases.
func Dedupe(vulns []*Vuln) []*Vuln {
	parent := map[string]string{}
	var find func(string) string
	find = func(id string) string {
		if p, ok := parent[id]; ok && p != id {
			parent[id] = find(p)
			return parent[id]
		}
		parent[id] = id
		return id
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for _, v := range vulns {
		for _, a := range v.Aliases {
			union(v.ID, a)
		}
	}
	groups := map[string][]*Vuln{}
	for _, v := range vulns {
		groups[find(v.ID)] = append(groups[find(v.ID)], v)
	}
	var out []*Vuln
	for _, g := range groups {
		slices.SortFunc(g, func(a, b *Vuln) int {
			ra, rb := a.DatabaseSpecific.Severity == "", b.DatabaseSpecific.Severity == ""
			if ra != rb {
				if ra {
					return 1
				}
				return -1
			}
			return strings.Compare(a.ID, b.ID)
		})
		keep := g[0]
		for _, other := range g[1:] {
			for _, id := range append([]string{other.ID}, other.Aliases...) {
				if id != keep.ID && !slices.Contains(keep.Aliases, id) {
					keep.Aliases = append(keep.Aliases, id)
				}
			}
		}
		out = append(out, keep)
	}
	slices.SortFunc(out, func(a, b *Vuln) int { return strings.Compare(a.ID, b.ID) })
	return out
}
