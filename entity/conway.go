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
	t.sc.Set(conwayLayer, 1) // TODO only cells made by NewConwayCell need this
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
	t.sc.Set(conwayLayer, 0)
	cellPool.Put(t)
}

func (t *ConwayCell) Act(ta *world.ActionAccumulator) {
	conwayLocal := t.sc.Look(conwayLayer)
	neighbors := game.CountNonZero(conwayLocal)
	// Conway's rules
	if neighbors <= 1 || neighbors >= 4 {
		// Die
		ta.Add(t.w.Now()+1, t.die, t.l.BlockId)
	} else { // 2 or 3 neighbors
		// Survive until next World tick
		ta.Add(t.w.Now()+2, t.Act, t.l.BlockId)
	}
	// Spawn new cell in an empty location if it has exactly 3 neighbors
	for d, nl := range t.l.Neighborhood() {
		d := game.Direction(d)
		if conwayLocal[d] != 0 {
			// A cell is already here
			continue
		}
		t.sc.Step(d)
		if game.CountNonZero(t.sc.Look(conwayLayer)) == 3 {
			c := cellPool.Get().(*ConwayCell)
			c.l = nl
			c.spawntick = t.w.Now() + 1
			ta.Spawn(c)
		}
		t.sc.Step(d.Reverse())
	}
}
