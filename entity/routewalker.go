package entity

import (
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"jds/game/world/path"
	"math/rand"
)

const PLAN_LENGTH = 4

type RouteWalker struct {
	id         world.EntityId
	w          *world.World
	l          game.Location
	sc         *layer.StackCursor
	route      path.Route
	dest       game.Location
	speed      float64
	intentions *layer.Layer
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
	t.route = path.NewRoute(w, t.l, t.dest)
	t.intentions = t.w.CustomLayer("RouteWalkerIntentions")
	if t.route.Len() > 0 {
		ta.Add(t.w.Now()+1, t, t.l.BlockId)
	}
}

func (t *RouteWalker) Touched(other world.EntityId, d game.Direction) {
}

func (t *RouteWalker) HitWall(d game.Direction) {
}

func (t *RouteWalker) Act(ta *game.ThoughtAccumulator) {

}
