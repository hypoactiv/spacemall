package entity

import (
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"jds/game/world/path"
	"math/rand"
)

type RouteWalker struct {
	id       world.EntityId
	w        *world.World
	l        game.Location
	sc       *layer.StackCursor
	dest     game.Location
	route    path.Route
	numSteps int
}

func NewRouteWalker(l game.Location, dest game.Location) *RouteWalker {
	return &RouteWalker{
		l:    l,
		dest: dest,
	}
}

func (t *RouteWalker) Location() game.Location {
	return t.l
}

func (t *RouteWalker) Spawned(ta *game.ThoughtAccumulator, id world.EntityId, w *world.World, sc *layer.StackCursor) {
	t.w = w
	t.id = id
	t.sc = sc
	t.route = path.NewRoute(w, t.l, t.dest)
	if t.route.Len() > 0 {
		ta.Add(t.w.Now()+1, t, t.l.BlockId)
	}
}

func (t *RouteWalker) Touched(other world.EntityId, d game.Direction) {
}

func (t *RouteWalker) HitWall(d game.Direction) {
}

func (t *RouteWalker) Act(ta *game.ThoughtAccumulator) {
	var tookStep bool
	var delay game.Ticks
	t.l, tookStep = t.w.StepEntity(t.id, t, t.sc, t.route.Direction(t.numSteps))
	if tookStep {
		t.numSteps++
	} else {
		t.l, tookStep = t.w.StepEntity(t.id, t, t.sc, game.Direction(rand.Intn(8)))
		if tookStep {
			t.numSteps = 0
			t.route = path.NewRoute(t.w, t.l, t.dest)
		}
		delay = game.Ticks(rand.Intn(100))
	}
	if t.numSteps < t.route.Len() {
		nt := ta.ExDirectWriteNextTick()
		nt.At = t.w.Now() + 1 + delay
		nt.Do = t
		nt.BlockId = t.l.BlockId
	}
}
