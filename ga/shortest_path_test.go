package ga

import (
	"testing"

	"github.com/go-ndn/nfd/graph"
)

func TestShortestPath(t *testing.T) {
	g := graph.New()
	g.AddUndirected("A", "C", uint64(2))
	g.AddUndirected("A", "B", uint64(4))
	g.AddUndirected("B", "C", uint64(5))

	g.AddUndirected("B", "D", uint64(10))
	g.AddUndirected("C", "E", uint64(3))

	g.AddUndirected("D", "E", uint64(4))
	g.AddUndirected("D", "F", uint64(11))

	r := ShortestPath(g, "A")
	for v, d := range map[string]uint64{
		"A": 0,
		"B": 4,
		"C": 2,
		"E": 5,
		"D": 9,
		"F": 20,
	} {
		if r[v] != d {
			t.Errorf("expected %s=%d, got %s=%d", v, d, v, r[v])
		}
	}
}
