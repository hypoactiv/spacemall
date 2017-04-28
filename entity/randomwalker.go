package entity

import (
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"math/rand"
)

type RandomWalker struct {
	id world.EntityId
	w  *world.World
	l  game.Location
	sc *layer.StackCursor
	// event counters
	spawned int
	touched int
	hitWall int
}

var randomTable [100]game.Direction
var numSteps int

func init() {
	for i := range randomTable {
		randomTable[i] = game.Direction(rand.Intn(8))
	}
}

func NewRandomWalker(l game.Location) *RandomWalker {
	return &RandomWalker{
		l: l,
	}
}

func (t *RandomWalker) Location() game.Location {
	return t.l
}

func (t *RandomWalker) Spawned(ta *world.ActionAccumulator, id world.EntityId, w *world.World, sc *layer.StackCursor) {
	t.w = w
	t.id = id
	t.sc = sc
	t.spawned++
	ta.Add(t.w.Now()+1, t.Act, t.l.BlockId)
}

func (t *RandomWalker) Touched(other world.EntityId, d game.Direction) {
	t.touched++
}

func (t *RandomWalker) HitWall(d game.Direction) {
	t.hitWall++
}

func (t *RandomWalker) Act(ta *world.ActionAccumulator) {
	numSteps++
	t.l, _ = t.w.StepEntity(t.id, t, t.sc, randomTable[numSteps%100])
	ta.Add(t.w.Now()+1, t.Act, t.l.BlockId)
}
