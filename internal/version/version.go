// Package version compares package versions and matches them against
// simple ranges like "<4.17.21" or ">=1.2.0 <1.3.0".
//
// ponytail: one comparison for every ecosystem. It follows semver
// (numbers compared as numbers, 1.0.0-rc.1 before 1.0.0) and accepts Go's
// leading v. PEP 440 oddities like post releases (1.0.post1) and epochs
// are compared as plain text segments, which orders the common cases
// right but not every one; use an exact version for those.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Compare returns -1, 0 or 1 as a is older than, the same as, or newer
// than b. Build metadata after "+" is ignored.
func Compare(a, b string) int {
	a, _, _ = strings.Cut(strings.TrimPrefix(a, "v"), "+")
	b, _, _ = strings.Cut(strings.TrimPrefix(b, "v"), "+")
	aRel, aPre, aHasPre := strings.Cut(a, "-")
	bRel, bPre, bHasPre := strings.Cut(b, "-")
	if c := compareParts(aRel, bRel); c != 0 {
		return c
	}
	switch {
	case aHasPre && !bHasPre:
		return -1
	case !aHasPre && bHasPre:
		return 1
	}
	return compareParts(aPre, bPre)
}

// compareParts compares dot separated parts, numerically when both are
// numbers. A missing part counts as 0, so 1.2 equals 1.2.0.
func compareParts(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		x, y := "0", "0"
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		xn, xerr := strconv.ParseUint(x, 10, 64)
		yn, yerr := strconv.ParseUint(y, 10, 64)
		switch {
		case xerr == nil && yerr == nil:
			if xn != yn {
				return cmp(xn < yn)
			}
		case xerr == nil: // numbers sort before words
			return -1
		case yerr == nil:
			return 1
		case x != y:
			return cmp(x < y)
		}
	}
	return 0
}

func cmp(less bool) int {
	if less {
		return -1
	}
	return 1
}

// Range is a set of conditions that must all hold.
type Range []cond

type cond struct {
	op      string
	version string
}

// ParseRange reads an exact version ("4.17.20", "=4.17.20") or one or more
// comparisons separated by spaces or commas (">=1.2.0 <1.3.0"). An empty
// string matches every version.
func ParseRange(s string) (Range, error) {
	var r Range
	for _, f := range strings.FieldsFunc(s, func(c rune) bool { return c == ' ' || c == ',' }) {
		op := ""
		for _, o := range []string{"<=", ">=", "==", "<", ">", "="} {
			if strings.HasPrefix(f, o) {
				op = o
				break
			}
		}
		v := strings.TrimPrefix(f, op)
		if op == "" || op == "==" {
			op = "="
		}
		if t := strings.TrimPrefix(v, "v"); t == "" || t[0] < '0' || t[0] > '9' {
			return nil, fmt.Errorf("version range %q: can't read %q (use =, <, <=, > or >= and a version)", s, f)
		}
		r = append(r, cond{op, v})
	}
	return r, nil
}

// Match reports whether v satisfies every condition in r.
func (r Range) Match(v string) bool {
	for _, c := range r {
		d := Compare(v, c.version)
		ok := false
		switch c.op {
		case "=":
			ok = d == 0
		case "<":
			ok = d < 0
		case "<=":
			ok = d <= 0
		case ">":
			ok = d > 0
		case ">=":
			ok = d >= 0
		}
		if !ok {
			return false
		}
	}
	return true
}
