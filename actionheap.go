package game

import (
	"container/heap"
)

type Action func(*ActionAccumulator)

type ScheduledAction struct {
	At      Tick
	Do      Action
	BlockId BlockId
}

type actionHeapInner struct {
	q []ScheduledAction
}

type ActionHeap struct {
	inner actionHeapInner
}

func (t *actionHeapInner) Len() int {
	return len(t.q)
}

func (t *actionHeapInner) Less(i int, j int) bool {
	return t.q[i].At < t.q[j].At
}

func (t *actionHeapInner) Swap(i int, j int) {
	t.q[i], t.q[j] = t.q[j], t.q[i]
}

func (t *actionHeapInner) Push(x interface{}) {
	t.q = append(t.q, x.(ScheduledAction))
}

func (t *actionHeapInner) Pop() (v interface{}) {
	last := len(t.q) - 1
	v, t.q = t.q[last], t.q[:last]
	return v
}

func (t *ActionHeap) Schedule(th ScheduledAction) { //at Tick, do Action, bid BlockId) {
	heap.Push(&t.inner, th)
}

// Returns the scheduled time of the next ScheduledAction to be returned by Next()
// Panics if heap is empty
func (t *ActionHeap) PeekTick() Tick {
	return t.inner.q[0].At
}

func (t *ActionHeap) PeekX() int {
	return t.inner.q[0].BlockId.X
}

func (t *ActionHeap) Next() ScheduledAction {
	if len(t.inner.q) == 0 {
		panic("ActionHeap empty")
	}
	return heap.Pop(&t.inner).(ScheduledAction)
}

func (t *ActionHeap) Len() int {
	return t.inner.Len()
}
