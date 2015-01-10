// Package pq implements priority queue
package pq

import (
	"container/heap"
)

type Item struct {
	Value    interface{} // The value of the item; arbitrary.
	Priority uint64      // The priority of the item in the queue.
	index    int         // The index of the item in the heap.
}

type PriorityQueue struct {
	m    map[interface{}]*Item
	item []*Item
}

func New() *PriorityQueue {
	return &PriorityQueue{
		m: make(map[interface{}]*Item),
	}
}

func (pq *PriorityQueue) Len() int { return len(pq.item) }

func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.item[i].Priority < pq.item[j].Priority
}

func (pq *PriorityQueue) Swap(i, j int) {
	pq.item[i], pq.item[j] = pq.item[j], pq.item[i]
	pq.item[i].index, pq.item[j].index = i, j
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*Item)
	if _, ok := pq.m[item.Value]; ok {
		return
	}
	item.index = pq.Len()
	pq.m[item.Value] = item
	pq.item = append(pq.item, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	item := pq.item[pq.Len()-1]
	delete(pq.m, item.Value)
	pq.item = pq.item[:pq.Len()-1]
	return item
}

func (pq *PriorityQueue) Update(x interface{}, priority uint64) {
	item, ok := pq.m[x]
	if !ok {
		return
	}
	item.Priority = priority
	heap.Fix(pq, item.index)
}
