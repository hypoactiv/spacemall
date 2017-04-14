package game

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestThinkHeap(t *testing.T) {
	N := 10000
	th := &ThinkHeap{}
	for i := 0; i < N; i++ {
		th.Schedule(Thought{At: Ticks(rand.Int())})
	}
	last := Ticks(-1)
	for th.Len() > 0 {
		w := th.PeekTicks()
		thought := th.Next()
		if last != -1 && last > w {
			t.Error("scheduled times not monotonic")
		}
		if thought.At != w {
			t.Error("peek returned wrong value")
		}
		last = w
	}
}

func BenchmarkThinkHeap(b *testing.B) {
	M := 100000
	b.StopTimer()
	fmt.Println(b.N)
	th := &ThinkHeap{}
	th.inner.q = make([]Thought, 0, M)
	ticks := make([]Ticks, M)
	for j := 0; j < M; j++ {
		ticks[j] = Ticks(rand.Int())
	}
	b.StartTimer()
	for j := 0; j < b.N/M; j++ {
		for i := 0; i < M; i++ {
			th.Schedule(Thought{At: ticks[i]})
		}
		for th.Len() > 0 {
			th.Next()
		}
	}
}
