package world

import (
	"fmt"
	"jds/game"
	"sort"
)

// Collects new ScheduledActions and sorts them by tick
// TODO rename WorkBuffer (?)
type ActionAccumulator struct {
	// Buffer actions for nextTick into slice NextTick
	nextTick game.Tick
	NextTick []ScheduledAction
	// Buffer all later actions into LaterTicks
	LaterTicks []ScheduledAction
	// Entity spawns and deaths happening before nextTick
	E struct {
		Spawns []Entity
		Deaths []EntityId
	}
	closed bool
}

func (aa *ActionAccumulator) AddAction(th ScheduledAction) {
	if aa.closed {
		panic("add to closed ActionAccumulator")
	}
	if th.At == aa.nextTick {
		aa.NextTick = append(aa.NextTick, th)
	} else if th.At > aa.nextTick {
		aa.LaterTicks = append(aa.LaterTicks, th)
	} else {
		fmt.Println(aa.nextTick, th.At)
		panic("action scheduled for past")
	}
}

func (aa *ActionAccumulator) Close() {
	if aa.closed {
		panic("closed already closed channel")
	}
	aa.closed = true
}

func (aa *ActionAccumulator) IsClosed() bool {
	return aa.closed
}

func (aa *ActionAccumulator) Add(at game.Tick, do Action, bid game.BlockId) {
	aa.AddAction(ScheduledAction{
		At:      at,
		Do:      do,
		BlockId: bid,
	})
}

func (aa *ActionAccumulator) Spawn(e Entity) {
	aa.E.Spawns = append(aa.E.Spawns, e)
}

func (aa *ActionAccumulator) Kill(e EntityId) {
	aa.E.Deaths = append(aa.E.Deaths, e)
}

func (aa *ActionAccumulator) Sort() {
	sort.Slice(aa.NextTick, func(i, j int) bool {
		return aa.NextTick[i].BlockId.X <= aa.NextTick[j].BlockId.X
	})
}

var aaPool []*ActionAccumulator

// Allocate an ActionAccumulator with nextTick t
func AllocateAA(t game.Tick) (aa *ActionAccumulator) {
	last := len(aaPool) - 1
	if last >= 0 {
		aa, aaPool = aaPool[last], aaPool[:last]
		aa.NextTick = aa.NextTick[:0]
		aa.LaterTicks = aa.LaterTicks[:0]
		aa.E.Deaths = aa.E.Deaths[:0]
		aa.closed = false
	} else {
		aa = new(ActionAccumulator)
	}
	aa.nextTick = t
	return
}

func ReleaseAA(aa *ActionAccumulator) {
	if aa == nil {
		return
	}
	// To garbage collect entities, must set slice values to nil
	for i := range aa.E.Spawns {
		aa.E.Spawns[i] = nil
	}
	aa.E.Spawns = aa.E.Spawns[:0]
	aaPool = append(aaPool, aa)
}
