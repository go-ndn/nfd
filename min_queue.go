package main

import (
	"container/heap"
)

type item struct {
	value    string // The value of the item; arbitrary.
	priority uint64 // The priority of the item in the queue.
	index    int    // The index of the item in the heap.
}

type minQueue struct {
	m    map[string]*item
	item []*item
}

func newMinQueue() *minQueue {
	return &minQueue{
		m: make(map[string]*item),
	}
}

func (this *minQueue) Len() int { return len(this.item) }

func (this *minQueue) Less(i, j int) bool {
	return this.item[i].priority < this.item[j].priority
}

func (this *minQueue) Swap(i, j int) {
	this.item[i], this.item[j] = this.item[j], this.item[i]
	this.item[i].index, this.item[j].index = i, j
}

func (this *minQueue) Push(x interface{}) {
	item := x.(*item)
	if _, ok := this.m[item.value]; ok {
		return
	}
	this.m[item.value] = item
	item.index = this.Len()
	this.item = append(this.item, item)
}

func (this *minQueue) Pop() interface{} {
	n := this.Len()
	item := this.item[n-1]
	delete(this.m, item.value)
	this.item = this.item[0 : n-1]
	return item
}

// update modifies the priority and value of an Item in the queue.
func (this *minQueue) update(value string, priority uint64) {
	item, ok := this.m[value]
	if !ok {
		return
	}
	item.priority = priority
	heap.Fix(this, item.index)
}
