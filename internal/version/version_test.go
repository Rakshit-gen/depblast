package version

import "testing"

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"4.17.20", "4.17.21", -1},
		{"4.17.9", "4.17.10", -1},
		{"1.2", "1.2.0", 0},
		{"v0.17.0", "0.17.0", 0},
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-alpha.2", "1.0.0-alpha.10", -1},
		{"1.0.0+build.5", "1.0.0", 0},
		{"v0.0.0-20230101000000-abcdef", "v0.0.0-20240101000000-123456", -1},
		{"2.0.0", "10.0.0", -1},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := Compare(c.b, c.a); got != -c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.b, c.a, got, -c.want)
		}
	}
}

func TestRange(t *testing.T) {
	for _, c := range []struct {
		r, v string
		want bool
	}{
		{"4.17.20", "4.17.20", true},
		{"4.17.20", "4.17.21", false},
		{"=4.17.20", "4.17.20", true},
		{"<4.17.21", "4.17.20", true},
		{"<4.17.21", "4.17.21", false},
		{">=1.2.0 <1.3.0", "1.2.9", true},
		{">=1.2.0, <1.3.0", "1.3.0", false},
		{"<=2.0.0", "2.0.0", true},
		{">v0.17.0", "v0.18.0", true},
		{"", "anything", true},
	} {
		r, err := ParseRange(c.r)
		if err != nil {
			t.Fatalf("%q: %v", c.r, err)
		}
		if got := r.Match(c.v); got != c.want {
			t.Errorf("%q matching %s = %v", c.r, c.v, got)
		}
	}
	for _, bad := range []string{"~1.2.0", "^4", ">=", "!=1.0"} {
		if _, err := ParseRange(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
