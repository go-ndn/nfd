package main

import (
	"github.com/taylorchu/ga"
	"github.com/taylorchu/graph"
	"github.com/taylorchu/ndn"
)

func bestRouteByName(state []*ndn.LSA, source string) map[string]ndn.Neighbor {
	// create graph from lsa dag
	g := graph.New()
	for _, v := range state {
		for _, u := range v.Neighbor {
			g.AddUndirected(v.Id, u.Id, u.Cost)
		}
	}
	// for each prefix, find a shortest neighbor to forward
	route := make(map[string]ndn.Neighbor)
	neighbors := bestRoute(g, source)
	for _, v := range state {
		if v.Id == source {
			continue
		}
		n := neighbors[v.Id]
		for _, name := range v.Name {
			if s, ok := route[name]; ok && n.Cost >= s.Cost {
				continue
			}
			route[name] = n
		}
	}
	return route
}

// for each neighbor, compute distance
func bestRoute(g graph.Graph, source graph.Vertex) map[graph.Vertex]ndn.Neighbor {
	route := make(map[graph.Vertex]ndn.Neighbor)
	// remove other links temperarily
	costs := g.GetEdges(source)
	for n, cost := range costs {
		g.RemoveVertex(source)
		g.AddUndirected(source, n, cost)
		for v, c := range ga.ShortestPath(g, source) {
			if u, ok := route[v]; ok && c >= u.Cost {
				continue
			}
			route[v] = ndn.Neighbor{
				Id:   n.(string),
				Cost: c,
			}
		}
	}
	// restore
	for n, cost := range costs {
		g.AddUndirected(source, n, cost)
	}
	return route
}
