package entity

import (
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"sync"
)

const conwayLayer = 2

// An entity that obeys the rules of Conway's Game of Life
type ConwayCell struct {
	id        world.EntityId
	w         *world.World
	l         game.Location
	sc        *layer.StackCursor
	spawntick game.Tick
}

var cellPool sync.Pool

func init() {
	cellPool.New = func() interface{} {
		return new(ConwayCell)
	}
}

func NewConwayCell(l game.Location) *ConwayCell {
	return &ConwayCell{
		l: l,
	}
}

func (t *ConwayCell) Location() game.Location {
	return t.l
}

func (t *ConwayCell) Spawned(ta *world.ActionAccumulator, id world.EntityId, w *world.World, sc *layer.StackCursor) {
	t.id = id
	t.w = w
	t.sc = sc
	if t.sc.Add(t.w.CustomLayer("Conway")) != conwayLayer {
		panic("unexpected layer id")
	}
	for d, v := range t.sc.Look(conwayLayer) {
		d := game.Direction(d)
		t.sc.DirectedSet(conwayLayer, d, v+1)
	}
	nexttick := (w.Now()/2 + 1) * 2
	ta.Add(nexttick, t.Act, t.l.BlockId)
	if w.Now() != t.spawntick && t.spawntick != 0 {
		panic("spawntick is wrong")
	}
}

func (t *ConwayCell) Touched(other world.EntityId, d game.Direction) {
}

func (t *ConwayCell) HitWall(d game.Direction) {
}

// 0 or 1, die
// 2 or 3, survive
// 4 or more, die
// exactly 3, spawn

func (t *ConwayCell) die(ta *world.ActionAccumulator) {
	ta.Kill(t.id)
	for d, v := range t.sc.Look(conwayLayer) {
		d := game.Direction(d)
		t.sc.DirectedSet(conwayLayer, d, v-1)
	}
	cellPool.Put(t)
}

func (t *ConwayCell) Act(ta *world.ActionAccumulator) {
	neighbors := t.sc.Get(conwayLayer)
	// Conway's rules
	if neighbors <= 1 || neighbors >= 4 {
		// Die
		ta.Add(t.w.Now()+1, t.die, t.l.BlockId)
	} else { // 2 or 3 neighbors
		// Survive until next World tick
		ta.Add(t.w.Now()+2, t.Act, t.l.BlockId)
	}
	// Spawn new cell in an empty neighboring location if it has exactly 3
	// neighboring cells
	conwayLocal := t.sc.Look(conwayLayer)
	entityLocal := t.sc.Look(0)
	for d, nl := range t.l.Neighborhood() {
		d := game.Direction(d)
		if entityLocal[d] != 0 {
			// Something, maybe another cell, is already here
			continue
		}
		if conwayLocal[d] == 3 {
			c := cellPool.Get().(*ConwayCell)
			c.l = nl
			c.spawntick = t.w.Now() + 1
			ta.Spawn(c)
		}
	}
}

func (t *ConwayCell) Color() game.Color {
	return game.Color{
		R: 255,
		G: 255,
		B: 255,
		A: 255,
	}
}
