package game

import (
	"container/heap"
)

type Actor interface {
	Act(*ThoughtAccumulator)
}

type Thought struct {
	At      Ticks
	Do      Actor
	BlockId BlockId
}

type thinkHeapInner struct {
	q []Thought
}

type ThinkHeap struct {
	inner thinkHeapInner
}

func (t *thinkHeapInner) Len() int {
	return len(t.q)
}

func (t *thinkHeapInner) Less(i int, j int) bool {
	return t.q[i].At < t.q[j].At
}

func (t *thinkHeapInner) Swap(i int, j int) {
	t.q[i], t.q[j] = t.q[j], t.q[i]
}

func (t *thinkHeapInner) Push(x interface{}) {
	t.q = append(t.q, x.(Thought))
}

func (t *thinkHeapInner) Pop() (v interface{}) {
	last := len(t.q) - 1
	v, t.q = t.q[last], t.q[:last]
	return v
}

func (t *ThinkHeap) Schedule(th Thought) { //at Ticks, do Action, bid BlockId) {
	heap.Push(&t.inner, th)
	/*heap.Push(&t.inner, Thought{
		At:      at,
		Do:      do,
		BlockId: bid,
	})*/
}

// Returns the next Thought scheduled time without altering the heap
// Panics if heap is empty
func (t *ThinkHeap) PeekTicks() Ticks {
	return t.inner.q[0].At
}

func (t *ThinkHeap) PeekX() int {
	return t.inner.q[0].BlockId.X
}

func (t *ThinkHeap) Next() Thought {
	if len(t.inner.q) == 0 {
		panic("thinkheap empty")
	}
	return heap.Pop(&t.inner).(Thought)
}

func (t *ThinkHeap) Len() int {
	return t.inner.Len()
}
