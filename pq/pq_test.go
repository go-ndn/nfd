package pq

import (
	"container/heap"
	"testing"
)

func TestPriorityQueue(t *testing.T) {
	items := map[string]uint64{
		"banana": 3, "apple": 2, "pear": 4,
	}

	q := New()
	for value, priority := range items {
		q.Push(&Item{
			Value:    value,
			Priority: priority,
		})
	}
	heap.Init(q)

	item := &Item{
		Value:    "orange",
		Priority: 1,
	}
	heap.Push(q, item)
	q.Update("orange", 5)

	expected := []string{"apple", "banana", "pear", "orange"}
	l := q.Len()
	for q.Len() > 0 {
		e := expected[l-q.Len()]
		item := heap.Pop(q).(*Item)
		if e != item.Value {
			t.Errorf("expected %s, got %s", e, item.Value)
		}
	}
}
