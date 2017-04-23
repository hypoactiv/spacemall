package game

import (
	"fmt"
	"sort"
)

type ActionAccumulator struct {
	NextTick   []ScheduledAction
	LaterTicks []ScheduledAction
	ticks      Tick // the earliest tick this AA will accept (these get buffered in NextTick)
}

func (aa *ActionAccumulator) AddAction(th ScheduledAction) {
	if th.At == aa.ticks {
		aa.NextTick = append(aa.NextTick, th)
	} else if th.At > aa.ticks {
		aa.LaterTicks = append(aa.LaterTicks, th)
	} else {
		fmt.Println(aa.ticks, th.At)
		panic("action scheduled for past")
	}
}

func (aa *ActionAccumulator) Add(at Tick, do Action, bid BlockId) {
	aa.AddAction(ScheduledAction{
		At:      at,
		Do:      do,
		BlockId: bid,
	})
}

// t is the earliest Tick this AA will accept
func (aa *ActionAccumulator) Reset(t Tick) {
	aa.ticks = t
	aa.NextTick = aa.NextTick[:0]
	aa.LaterTicks = aa.LaterTicks[:0]
}

func (aa *ActionAccumulator) Sort() {
	sort.Slice(aa.NextTick, func(i, j int) bool {
		return aa.NextTick[i].BlockId.X <= aa.NextTick[j].BlockId.X
	})
}

var aaPool []*ActionAccumulator

func AllocateAA(t Tick) (aa *ActionAccumulator) {
	last := len(aaPool) - 1
	if last >= 0 {
		aa, aaPool = aaPool[last], aaPool[:last]
	} else {
		aa = new(ActionAccumulator)
	}
	aa.Reset(t)
	return

}

func ReleaseAA(aa *ActionAccumulator) {
	if aa == nil {
		return
	}
	aaPool = append(aaPool, aa)
}
