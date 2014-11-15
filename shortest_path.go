package main

import (
	"github.com/taylorchu/ga"
	"github.com/taylorchu/graph"
	"github.com/taylorchu/ndn"
)

func computeNextHop(source string, state []*ndn.LSA) map[string]ndn.Neighbor {
	// create graph from lsa dag
	g := graph.New()
	for _, v := range state {
		for _, u := range v.Neighbor {
			g.AddUndirected(v, u, u.Cost)
		}
	}
	// for each prefix, find a shortest neighbor to forward
	shortest := make(map[string]ndn.Neighbor)
	for n, costs := range computeMultiPath(g, source) {
		for _, v := range state {
			if v.Id == source {
				continue
			}
			cost := costs[v.Id]
			for _, name := range v.Name {
				if s, ok := shortest[name]; ok && cost >= s.Cost {
					continue
				}
				shortest[name] = ndn.Neighbor{
					Id:   n.(string),
					Cost: cost,
				}
			}
		}
	}
	return shortest
}

// for each neighbor, compute distance if that face is chosen
func computeMultiPath(g graph.Graph, source graph.Vertex) map[graph.Vertex]map[graph.Vertex]uint64 {
	nextHopCost := make(map[graph.Vertex]map[graph.Vertex]uint64)
	// remove other links temperarily
	costs := g.GetEdges(source)
	for n, cost := range costs {
		g.RemoveVertex(source)
		g.AddUndirected(source, n, cost)
		nextHopCost[n] = ga.ShortestPath(g, source)
	}
	// restore
	for n, cost := range costs {
		g.AddUndirected(source, n, cost)
	}
	return nextHopCost
}
