package ga

import (
	"container/heap"
	"math"

	"github.com/go-ndn/nfd/graph"
	"github.com/go-ndn/nfd/pq"
)

// dijkstra with priority queue. distance is strictly positive.
func ShortestPath(g graph.Graph, s graph.Vertex) map[graph.Vertex]uint64 {
	dist := make(map[graph.Vertex]uint64)
	scanned := make(map[graph.Vertex]struct{})
	q := pq.New()
	for _, v := range g.Vertices() {
		if v == s {
			dist[v] = 0
		} else {
			dist[v] = math.MaxUint64
		}
		q.Push(&pq.Item{
			Value:    v,
			Priority: dist[v],
		})
	}
	heap.Init(q)
	for q.Len() > 0 {
		u := heap.Pop(q).(*pq.Item).Value
		scanned[u] = struct{}{}
		for v, d := range g.Edges(u) {
			if _, ok := scanned[v]; ok {
				continue
			}
			alt := dist[u] + d.(uint64)
			if alt >= dist[v] {
				continue
			}
			dist[v] = alt
			q.Update(v, alt)
		}
	}
	return dist
}
