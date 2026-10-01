// Package store keeps dependency graphs in HydraDB.
//
// Each indexed lockfile becomes a Project vertex, and each package version
// it locks becomes a Pkg vertex of its own, joined by DEPENDS_ON edges.
// Packages are not shared between projects: the same version can depend
// on different versions in two projects (npm resolves every tree on its
// own), so a shared vertex would let paths cross from one project into
// another. Finding every project that ships lodash 4.17.20 is a property
// lookup on name and version instead.
package store

import (
	"hash/fnv"
	"strconv"

	"github.com/Rakshit-gen/depblast/internal/dep"
)

// HydraDB vertex ids are unsigned integers. They are kept below 2^63 so
// they also fit a signed integer on every client.
func hashID(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return h.Sum64() &^ (1 << 63)
}

// ProjectID is the vertex id of the project read from a lockfile, given
// as an absolute path.
func ProjectID(lockfile string) uint64 { return hashID("project", lockfile) }

// PkgID is the vertex id of a package version inside one project.
func PkgID(project uint64, p dep.Pkg) uint64 {
	return hashID("pkg", strconv.FormatUint(project, 10), p.Eco, p.Name, p.Version)
}

func edgeID(from, to uint64) uint64 {
	return hashID("edge", strconv.FormatUint(from, 10), strconv.FormatUint(to, 10))
}
