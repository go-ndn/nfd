package main

import (
	"container/heap"
	"github.com/taylorchu/ndn"
	"math"
)

type distMap map[string]uint64

// TODO: multipath, currently only choose the best
func computeNextHop(source string, state []*ndn.LSA) map[string]ndn.Neighbor {
	// create graph from lsa dag
	graph := make(map[string]distMap)
	for _, v := range state {
		if _, ok := graph[v.Id]; !ok {
			graph[v.Id] = make(distMap)
		}
		for _, u := range v.Neighbor {
			graph[v.Id][u.Id] = u.Cost
			if _, ok := graph[u.Id]; !ok {
				graph[u.Id] = make(distMap)
			}
			graph[u.Id][v.Id] = u.Cost
		}
	}
	// for each prefix, find a shortest neighbor to forward
	shortest := make(map[string]ndn.Neighbor)
	for n, dist := range computeMultiPath(source, graph) {
		for _, v := range state {
			if v.Id == source {
				continue
			}
			cost := dist[v.Id]
			for _, name := range v.Name {
				if s, ok := shortest[name]; ok && cost >= s.Cost {
					continue
				}
				shortest[name] = ndn.Neighbor{
					Id:   n,
					Cost: cost,
				}
			}
		}
	}
	return shortest
}

// for each neighbor, compute distance if that face is chosen
func computeMultiPath(source string, graph map[string]distMap) map[string]distMap {
	shortest := make(map[string]distMap)
	// remove other links temperarily
	dists := graph[source]
	for n, cost := range dists {
		graph[source] = distMap{n: cost}
		for n := range dists {
			delete(graph[n], source)
		}
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
