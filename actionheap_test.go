package game

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestActionHeap(t *testing.T) {
	N := 10000
	th := &ActionHeap{}
	for i := 0; i < N; i++ {
		th.Schedule(ScheduledAction{At: Tick(rand.Int())})
	}
	last := Tick(-1)
	for th.Len() > 0 {
		w := th.PeekTick()
		action := th.Next()
		if last != -1 && last > w {
			t.Error("scheduled times not monotonic")
		}
		if action.At != w {
			t.Error("peek returned wrong value")
		}
		last = w
	}
}

func BenchmarkActionHeap(b *testing.B) {
	M := 100000
	b.StopTimer()
	fmt.Println(b.N)
	th := &ActionHeap{}
	th.inner.q = make([]ScheduledAction, 0, M)
	ticks := make([]Tick, M)
	for j := 0; j < M; j++ {
		ticks[j] = Tick(rand.Int())
	}
	b.StartTimer()
	for j := 0; j < b.N/M; j++ {
		for i := 0; i < M; i++ {
			th.Schedule(ScheduledAction{At: ticks[i]})
		}
		for th.Len() > 0 {
			th.Next()
		}
	}
}
