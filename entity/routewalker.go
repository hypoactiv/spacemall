package entity

import (
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"jds/game/world/path"
	"math/rand"
)

type RouteWalker struct {
	id             world.EntityId
	w              *world.World
	l              game.Location
	sc             *layer.StackCursor
	routeCursor    game.Location
	routeCursorLoc int
	route          path.Route
	dest           game.Location
	speed          float64
	expectTick     game.Ticks
	lastDirection  game.Direction
}

const (
	entityIndex = 0
	wallIndex   = 1
)

func NewRouteWalker(l game.Location, dest game.Location) *RouteWalker {
	return &RouteWalker{
		l:     l,
		dest:  dest,
		speed: rand.Float64()*0.8 + 0.2,
	}
}

func (t *RouteWalker) Location() game.Location {
	return t.l
}

func (t *RouteWalker) Spawned(ta *game.ThoughtAccumulator, id world.EntityId, w *world.World, sc *layer.StackCursor) {
	t.w = w
	t.id = id
	t.sc = sc
	t.routeCursor = t.l
	t.routeCursorLoc = 0
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
	bestDirection := game.Direction(game.NONE)
	bestDistToCursor := t.l.MaxDistance(t.routeCursor)
	if bestDistToCursor < 2 {
		if t.routeCursorLoc < t.route.Len()-1 {
			t.routeCursorLoc++
			t.routeCursor = t.routeCursor.JustStep(t.route.Direction(t.routeCursorLoc))
			bestDistToCursor = t.l.MaxDistance(t.routeCursor)
		}
	}
	for d, l := range t.l.Neighborhood() {
		d := game.Direction(d)
		if t.sc.DirectedGet(entityIndex, d) != 0 {
			continue
		}
		dist := l.MaxDistance(t.routeCursor)
		if dist <= bestDistToCursor {
			bestDistToCursor = dist
			bestDirection = d
		}
	}
	if bestDirection != game.NONE {
		t.l, _ = t.w.StepEntity(t.id, t, t.sc, bestDirection)
	} else if t.lastDirection != game.NONE {
		// no good move possible
		if t.routeCursorLoc < t.route.Len()-1 {
			t.routeCursorLoc++
			t.routeCursor = t.routeCursor.JustStep(t.route.Direction(t.routeCursorLoc))
		}
	}
	t.lastDirection = bestDirection
	var delay game.Ticks
	delay = 1
	for rand.Float64() < 1-t.speed {
		delay++
	}
	//fmt.Println(delay)
	ta.Add(t.w.Now()+1+delay, t, t.l.BlockId)
	t.expectTick = t.w.Now() + 1 + delay
	/*
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
	*/
}
