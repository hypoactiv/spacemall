package world

import (
	"jds/game"
	"jds/game/layer"
)

type EntityId int

const ENTITYID_INVALID = 0

type Entity interface {
	// Entity E's location
	Location() game.Location
	// Events happening to an Entity 'E'
	//
	// E has spawned into world w with EntityId id. sc is a StackCursor
	// with w.EntityIds and w.Walls as layers 0 and 1, respectively, and
	// cursor position at E.Location()
	Spawned(ta *game.ActionAccumulator, id EntityId, w *World, sc *layer.StackCursor)
	// E has attempted to move to other's location, or other has attempted to
	// move to E's location. 'd' is the direction of 'other' relative to E.
	Touched(otherEid EntityId, d game.Direction)
	// E attempted to move to the wall in direction 'd'
	HitWall(d game.Direction)
}
