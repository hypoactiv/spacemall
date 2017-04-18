package game

import (
	"fmt"
	"sort"
)

type ThoughtAccumulator struct {
	NextTick   []Thought
	LaterTicks []Thought
	ticks      Ticks // the earliest tick this TA will accept (these get buffered in NextTick)
}

func (ta *ThoughtAccumulator) AddThought(th Thought) {
	if th.At == ta.ticks {
		ta.NextTick = append(ta.NextTick, th)
	} else if th.At > ta.ticks {
		ta.LaterTicks = append(ta.LaterTicks, th)
	} else {
		fmt.Println(ta.ticks, th.At)
		panic("thought scheduled for past")
	}
}

func (ta *ThoughtAccumulator) ExDirectWriteNextTick() *Thought {
	ta.NextTick = append(ta.NextTick, Thought{})
	return &ta.NextTick[len(ta.NextTick)-1]
}

func (ta *ThoughtAccumulator) Add(at Ticks, do Actor, bid BlockId) {
	ta.AddThought(Thought{
		At:      at,
		Do:      do,
		BlockId: bid,
	})
}

// t is the earliest Tick this TA will accept
func (ta *ThoughtAccumulator) Reset(t Ticks) {
	ta.ticks = t
	ta.NextTick = ta.NextTick[:0]
	ta.LaterTicks = ta.LaterTicks[:0]
}

func (ta *ThoughtAccumulator) Sort() {
	sort.Slice(ta.NextTick, func(i, j int) bool {
		return ta.NextTick[i].BlockId.X <= ta.NextTick[j].BlockId.X
	})
}

var taPool []*ThoughtAccumulator

func AllocateTA(t Ticks) (ta *ThoughtAccumulator) {
	last := len(taPool) - 1
	if last >= 0 {
		ta, taPool = taPool[last], taPool[:last]
	} else {
		ta = new(ThoughtAccumulator)
	}
	ta.Reset(t)
	return

}

func ReleaseTA(ta *ThoughtAccumulator) {
	if ta == nil {
		return
	}
	taPool = append(taPool, ta)
}
