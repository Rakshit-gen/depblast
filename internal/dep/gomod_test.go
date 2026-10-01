package dep

import (
	"slices"
	"strings"
	"testing"
)

func gomod(path, version string) Pkg { return Pkg{Go, path, version} }

func TestParseGoGraphKeepsSelectedVersions(t *testing.T) {
	graph := `example.com/app golang.org/x/net@v0.17.0
example.com/app github.com/gin-gonic/gin@v1.9.0
github.com/gin-gonic/gin@v1.9.0 golang.org/x/net@v0.10.0
github.com/gin-gonic/gin@v1.9.0 github.com/ugorji/go/codec@v1.2.11
golang.org/x/net@v0.10.0 golang.org/x/text@v0.9.0
golang.org/x/net@v0.17.0 golang.org/x/text@v0.13.0
golang.org/x/text@v0.9.0 golang.org/x/tools@v0.6.0
`
	list := `example.com/app
github.com/gin-gonic/gin v1.9.0
github.com/ugorji/go/codec v1.2.11
golang.org/x/net v0.17.0
golang.org/x/text v0.13.0
`
	g, err := ParseGoGraph("go.mod", strings.NewReader(graph), strings.NewReader(list))
	if err != nil {
		t.Fatal(err)
	}
	gin, net := gomod("github.com/gin-gonic/gin", "v1.9.0"), gomod("golang.org/x/net", "v0.17.0")
	if !slices.Equal(g.Direct, []Pkg{net, gin}) {
		t.Fatalf("direct %v", g.Direct)
	}
	// gin asked for net v0.10.0, but the build uses v0.17.0.
	if !slices.Contains(g.Deps[gin], net) {
		t.Fatalf("gin deps %v", g.Deps[gin])
	}
	// x/net v0.10.0 lost, so its requirement on x/text v0.9.0 (and that
	// version's on x/tools) is dropped.
	if d := g.Deps[net]; !slices.Equal(d, []Pkg{gomod("golang.org/x/text", "v0.13.0")}) {
		t.Fatalf("net deps %v", d)
	}
	if len(g.Packages()) != 4 {
		t.Fatalf("packages %v", g.Packages())
	}
}
