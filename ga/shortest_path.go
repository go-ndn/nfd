package ga

import (
	"container/heap"
	"math"

	"github.com/go-ndn/nfd/graph"
	"github.com/go-ndn/nfd/pq"
)

// dijkstra with priority queue. distance is strictly positive.
func ShortestPath(g graph.Graph, s graph.Vertex) map[graph.Vertex]uint64 {
	dist := map[graph.Vertex]uint64{s: 0}
	scanned := make(map[graph.Vertex]bool)
	q := pq.New()
	for _, v := range g.Vertices() {
		if v != s {
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
		scanned[u] = true
		for v, d := range g.Edges(u) {
			if scanned[v] {
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