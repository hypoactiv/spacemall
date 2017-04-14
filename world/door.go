package world

import (
	"fmt"
	"jds/game"
	"jds/game/patterns"
)

type Orientation bool

const (
	HORZ = true
	VERT = false
)

type DoorId int

type Door struct {
	Id DoorId
	O  Orientation
	L  game.Location
	R  [2]RoomId // The RoomId's on either side of the door
	w  *World
}

func (w *World) NewDoor(l game.Location, o Orientation, m game.ModMap) (d *Door) {
	d = &Door{
		Id: w.nextDoorId,
		O:  o,
		L:  l,
		w:  w,
	}
	if !w.CanPlaceDoor(l, o) {
		return nil
	}
	w.nextDoorId++
	w.DoorIds.SetMask(d.L, patterns.DoorId, d.transpose(), game.TileId(d.Id), m)
	d.updateRids()
	for _, rid := range d.R {
		if rid == 0 {
			continue
		}
		r := w.Rooms[rid]
		r.addDoorId(d.Id)
	}
	w.Doors[d.Id] = d
	return
}

func (d *Door) updateRids() {
	for i, l := range d.DoorSteps() {
		rid := RoomId(d.w.RoomIds.Get(l))
		d.R[i] = rid
	}
}

func (w *World) DumpDoors() {
	for _, v := range w.Doors {
		fmt.Println(v)
	}
}

func (d *Door) String() string {
	return fmt.Sprintf("(%d)[%d:%d]", d.Id, d.R[0], d.R[1])
}

// Returns the adjacent rooms. May be nil.
func (d *Door) Rooms() [2]*Room {
	return [2]*Room{
		d.w.Rooms[d.R[0]],
		d.w.Rooms[d.R[1]],
	}
}

// Returns Locations in each of the two rooms joined by d
func (d *Door) DoorSteps() [2]game.Location {
	switch d.O {
	case VERT:
		return [2]game.Location{
			d.L.JustOffset(0, 0),
			d.L.JustOffset(2, 0),
		}
	case HORZ:
		return [2]game.Location{
			d.L.JustOffset(0, 0),
			d.L.JustOffset(0, 2),
		}
	default:
		panic("invalid orientation")
	}
}

// maps door orientation to pattern transpose flag
func (d *Door) transpose() (transpose bool) {
	if d.O == VERT {
		transpose = false
	} else if d.O == HORZ {
		transpose = true
	} else {
		panic("invalid door orientation")
	}
	return
}

// return true if door can be placed into the world at l with orientation o
func (w *World) CanPlaceDoor(l game.Location, o Orientation) bool {
	transpose := false
	if o == HORZ {
		transpose = true
	}
	return w.Walls.Match(l, patterns.Door, transpose) &&
		w.DoorIds.Match(l, patterns.Zero4x3, transpose)
}

// Verifies consistency of Door
func (d *Door) fsck() {
	// Ensure underlying wall pattern is valid
	if !d.w.Walls.Match(d.L, patterns.Door, d.transpose()) {
		panic("walls around door inconsistent")
	}
	// Ensure DoorIds are set
	if !d.w.DoorIds.Match(d.L, patterns.DoorId.Remap(game.TileId(d.Id)), d.transpose()) {
		panic("DoorId inconsistent")
	}
	// Ensure RoomIds are consistent
	for i, l := range d.DoorSteps() {
		if got, actual := d.R[i], RoomId(d.w.RoomIds.Get(l)); got != actual {
			fmt.Printf("got %d actual %d\n", got, actual)
			panic("Door's stored RoomIds are inconsistent with RoomIds layer")
		}
	}
	// Ensure connected rooms reference back to this Door
	for _, rid := range d.R {
		if rid == 0 {
			continue
		}
		r := d.w.Rooms[rid]
		for _, did := range r.DoorIds {
			if did == d.Id {
				goto okay
			}
		}
		// missing room->door reference
		// see if some other room has a reference to this door
		for rid, r := range d.w.Rooms {
			for _, did := range r.DoorIds {
				if did == d.Id {
					fmt.Printf("rid: %d has reference to did %d\n", rid, d.Id)
				}
			}
		}
		fmt.Printf("rid: %d, did: %d d.R: %v r.DoorIds: %v\n", r.id, d.Id, d.R, r.DoorIds)
		d.updateRids()
		fmt.Printf("rid: %d, did: %d d.R: %v r.DoorIds: %v\n", r.id, d.Id, d.R, r.DoorIds)
		panic("Missing Room->Door reference")
	okay:
	}
}

func (d *Door) Delete(m game.ModMap) {
	// Clear DoorIds layer
	d.w.DoorIds.SetMask(d.L, patterns.DoorId, d.transpose(), 0, m)
	// Remove from adjacent rooms
	for _, room := range d.Rooms() {
		if room == nil {
			continue
		}
		room.removeDoorId(d.Id)
	}
	// Remove from world
	delete(d.w.Doors, d.Id)
}
