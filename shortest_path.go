package main

import (
	"container/heap"
	"math"
)

type distMap map[string]uint64

// for each neighbor, compute distance if that face is chosen
func computeMultiPath(source string, graph map[string]distMap) map[string]distMap {
	shortest := make(map[string]distMap)
	// remove other links temperarily
	dists := graph[source]
	for n, cost := range dists {
		delete(graph, source)
		for n := range dists {
			delete(graph[n], source)
		}
		graph[source] = distMap{n: cost}
		graph[n][source] = cost
		shortest[n] = computeSinglePath(source, graph)
	}
	// restore
	graph[source] = dists
	for n, cost := range dists {
		graph[n][source] = cost
	}
	return shortest
}

// dijkstra with priority queue
func computeSinglePath(source string, graph map[string]distMap) distMap {
	dist := distMap{source: 0}
	scanned := make(map[string]bool)
	q := newMinQueue()
	for v := range graph {
		if v != source {
			dist[v] = math.MaxUint64
		}
		q.Push(&item{
			value:    v,
			priority: dist[v],
		})
	}
	heap.Init(q)
	for q.Len() > 0 {
		u := heap.Pop(q).(*item).value
		scanned[u] = true
		for v, l := range graph[u] {
			if scanned[v] {
				continue
			}
			alt := dist[u] + l
			if alt >= dist[v] {
				continue
			}
			dist[v] = alt
			q.update(v, alt)
		}
	}
	return dist
}
