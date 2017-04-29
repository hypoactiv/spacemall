package world

import (
	"fmt"
	"jds/game"
	"sort"
)

// Collects new ScheduledActions and sorts them by tick
type ActionAccumulator struct {
	// Buffer actions for nextTick into slice NextTick
	nextTick game.Tick
	NextTick []ScheduledAction
	// Buffer all later actions into LaterTicks
	LaterTicks []ScheduledAction
	// Entity spawns and deaths happening during nextTick
	E struct {
		Spawns []Entity
		Deaths []EntityId
	}
}

func (aa *ActionAccumulator) AddAction(th ScheduledAction) {
	if th.At == aa.nextTick {
		aa.NextTick = append(aa.NextTick, th)
	} else if th.At > aa.nextTick {
		aa.LaterTicks = append(aa.LaterTicks, th)
	} else {
		fmt.Println(aa.nextTick, th.At)
		panic("action scheduled for past")
	}
}

func (aa *ActionAccumulator) Add(at game.Tick, do Action, bid game.BlockId) {
	aa.AddAction(ScheduledAction{
		At:      at,
		Do:      do,
		BlockId: bid,
	})
}

// t is the earliest Tick this AA will accept
func (aa *ActionAccumulator) Reset(t game.Tick) {
	aa.nextTick = t
	aa.NextTick = aa.NextTick[:0]
	aa.LaterTicks = aa.LaterTicks[:0]
}

func (aa *ActionAccumulator) Sort() {
	sort.Slice(aa.NextTick, func(i, j int) bool {
		return aa.NextTick[i].BlockId.X <= aa.NextTick[j].BlockId.X
	})
}

var aaPool []*ActionAccumulator

func AllocateAA(t game.Tick) (aa *ActionAccumulator) {
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
