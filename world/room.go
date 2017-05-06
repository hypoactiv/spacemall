package world

import (
	"fmt"
	"jds/game"
	"jds/game/layer"
	"sort"
)

type RoomId int

const (
	ROOMID_INVALID = 0
)

type Room struct {
	id         RoomId
	ApproxArea int
	Area       int
	// Location of the linking node (LN) for Room
	LNL game.Location
	// Direction from LN to LN's parent
	LNPD game.Direction
	// Doors connected to this room
	DoorIds []DoorId
	// The World that contains this room
	w *World
}

// Linking node's parent's location
func (r *Room) LNPL() (lnpl game.Location) {
	lnpl = r.LNL.JustStep(r.LNPD)
	return
}

// Fetches and returns the Linking Node's parent
// TODO maybe faster to just store this in Room?
func (r *Room) LNP() *WallTreeNode {
	return r.w.WallNodes[r.LNPL()] // step from LN to LNP and return
}

// Reconstruct the linking node that was discovered when this room was created
func (r *Room) LinkingNode() (n WallTreeNode) {
	n = WallTreeNode{
		L: r.LNL,
		D: r.LNPD,
		P: r.LNP(),
	}
	n.R = n.P.R
	n.Depth = n.P.Depth + 1
	return
}

// Set RoomId for all interior tiles
func (r *Room) init(m game.ModMap) {
	var room *Room
	r.Area = 0
	replacing := make(map[RoomId]int)
	replacingRid := RoomId(ROOMID_INVALID)
	seenDids := make(map[DoorId]struct{})
	// Iterate over rows of the room
	r.paint(func(rm *game.RowMask, rid []game.TileId) bool {
		// Set RoomId of a row of the room
		runLength := 0 // Run length of common RoomIds being replaced with r.id
		cursor := rm.Left
		// Look for doors on this row, and remove them from their original rooms
		doorIds, doorDists := r.w.DoorIds.CollectRowMask(rm)
		for i, did := range doorIds {
			did := DoorId(did)
			seenDids[did] = struct{}{}
			roomid := RoomId(rid[doorDists[i]])
			if roomid == ROOMID_INVALID {
				// no existing room on this side of the door
				continue
			}
			origRoom := r.w.Rooms[roomid]
			if origRoom == nil {
				//deleted room
				continue
			}
			// door with id 'did' is connected to room 'origRoom', wich is about to be
			// recolored to room 'r'
			// Remove 'did' from 'origRoom' and connect it to 'r' later
			origRoom.removeDoorId(did) // does nothing if the door was already removed
		}
		for i := 0; i < rm.Width(); {
			paint, skip := rm.Mask(i)
			if paint {
				// Paint on
				for j := 0; j < skip; j++ {
					if RoomId(rid[i]) != replacingRid {
						// TODO move this into a first-class function since it gets repeated below
						// End of RoomId run, update Area count
						if replacingRid != r.id {
							replacing[replacingRid] += runLength
							room = r.w.Rooms[replacingRid]
							if room != nil {
								if room.id != replacingRid {
									panic("replacing rid wrong")
								}
								room.Area -= runLength
							}
						}
						runLength = 1
						replacingRid = RoomId(rid[i])
					} else {
						runLength++
					}
					r.Area++
					cursor, _, _ = cursor.Right()
					i++
				}
			} else {
				// Paint off
				if runLength > 0 {
					// End of RoomId run, update Area count
					if replacingRid != r.id {
						replacing[replacingRid] += runLength
						room = r.w.Rooms[replacingRid]
						if room != nil {
							room.Area -= runLength
						}
					}
					runLength = 0
					room = nil
					replacingRid = ROOMID_INVALID
				}
				i += skip
				cursor, _, _ = cursor.Offset(skip, 0)
			}
		}
		// Set all tiles in RowMask rm to have RoomId r.id
		r.w.RoomIds.SetRowMask(rm, game.TileId(r.id), m)
		// recycle rid slice to load DoorId's in this row
		return true
	})
	r.DoorIds = make([]DoorId, 0, len(seenDids))
	for did := range seenDids {
		r.addDoorId(did)
		d := r.w.Doors[did]
		d.updateRids()
		// Update adjacent rooms' door references
		for _, doorRoom := range d.Rooms() {
			if doorRoom == nil {
				continue
			}
			doorRoom.addDoorId(d.Id)
		}
	}
	if len(replacing) == 1 {
		// Only 1 RoomId was replaced
		var original RoomId
		for original = range replacing {
		}
		// 'original' contains the single RoomId that was replaced during this recolor
		// Should a RoomId change be done to change r.id to 'original'?
		if original == ROOMID_INVALID {
			// Don't do change
			return
		}
		originalRoom := r.w.Rooms[original]
		if originalRoom != nil {
			if originalRoom.Area+replacing[original] > r.Area {
				// original room still has tiles somewhere else, don't do change
				// as this produce a room with >1 connected components.
				return
			}
			// If we reach here, the original room must have been replaced entirely and
			// so no Area inside it anymore. Verify this.
			if originalRoom.Area != 0 {
				layerError := layer.NewLayerFromSlice(r.w.RoomIds.DeepSearch(game.TileId(original)), game.TileId(original))
				layerError.SetSlice(r.w.RoomIds.DeepSearch(game.TileId(r.id)), game.TileId(r.id))
				panic(LayerError{
					Layer:   layerError,
					Message: "didn't replace all original tiles",
				})
			}
			// Delete the old, replaced room
			delete(originalRoom.LNP().RoomIds, original)
			delete(r.w.Rooms, original)
		}
		// Schedule r.id to be changed to 'original'
		r.w.scheduleRoomIdRemap(r.id, original)
	} else {
		// try other room id changes here, to minimizing number of rooms chaning ID's
	}
	return
}

func (w *World) scheduleRoomIdRemap(old, new RoomId) {
	w.roomIdRemapStack = append(w.roomIdRemapStack, roomIdRemap{Old: old, New: new})
}

// Checks if a room is empty. If it is not, return true and an arbitrary location inside the room.
func (r *Room) IsNonEmpty() (nonempty bool, loc game.Location) {
	// TODO this is really awful. check the neighborhood of the linking node to see if there's a room tile there
	if r.Area == 0 {
		nonempty = false
		return
	}
	for _, n := range r.LNL.Neighborhood() {
		if RoomId(r.w.RoomIds.Get(n)) == r.id {
			return true, n
		}
	}
	panic("no interior tile found")
}

// Iterate f over thw rows of 'r'
func (r *Room) Interior(f func(*game.RowMask) bool) {
	n1 := r.LinkingNode()
	n2 := r.w.WallNodes[n1.L]
	ca := commonAncestor(&n1, n2)
	left, width, maxWidth := computeLeftAndRightMost(&n1, n2, ca)
	ridRow := make([]game.TileId, maxWidth)
	rowMask := game.NewRowMask(maxWidth)
	sc := layer.NewStackCursor(left[0])
	liRids := sc.Add(r.w.RoomIds)
	for i, l := range left {
		rowMask.Reset()
		rowMask.Left = l
		sc.MoveTo(l)
		sc.GetRow(liRids, ridRow[:width[i]])
		for j := 0; j < width[i]; j++ {
			rowMask.Append(RoomId(ridRow[j]) == r.id)
		}
		if !f(rowMask) {
			break
		}
	}
}

// Iterates the function f over the tiles of Room r. The arguments passed to f
// are a RowMask describing the tiles of the current row belonging to Room r,
// and an []int containing the original RoomId's stored in the row. This slice
// is not []RoomId so that it can be reused to read other data from the
// World.
func (r *Room) paint(f func(*game.RowMask, []game.TileId) bool) {
	// These are the nodes that were discovered in addToWallTree, when r was created
	n1 := r.LinkingNode()
	n2 := r.w.WallNodes[n1.L]
	// Sanity check
	if n2 == nil || n1.R != n2.R {
		fmt.Println(n1, n2)
		fmt.Println("link directions for", r.id, r.LNP().RoomIds)
		fmt.Println("ERROR linking nodes are from different trees. shouldn't happen")
		//fmt.Println("couldn't find rid", r.id, "linkage at", r.N.Step(r.D))
		panic("asdf")
	}
	ca := commonAncestor(&n1, n2)
	// There are unique paths from n1 to ca and n2 to ca. Since n1.L == n2.L,
	// n1->ca->n2 forms a loop
	var tangentLayer *layer.Layer
	tangentLayer, r.ApproxArea = computeTangent(&n1, n2, ca)
	defer tangentLayer.Discard()
	left, width, maxWidth := computeLeftAndRightMost(&n1, n2, ca)
	// Build stack cursor
	sc := layer.NewStackCursor(left[0])
	liWalls := sc.Add(r.w.Walls)
	liTangent := sc.Add(tangentLayer)
	liRids := sc.Add(r.w.RoomIds)
	// Allocate row buffers
	wallRow := make([]game.TileId, maxWidth)
	tangentRow := make([]game.TileId, maxWidth)
	ridRow := make([]game.TileId, maxWidth)
	rowMask := game.NewRowMask(maxWidth)
	for y, leftmost := range left {
		var existingRoom *Room
		rowMask.Reset()
		rowMask.Left = leftmost
		lastRid := RoomId(ROOMID_INVALID)
		paint := false
		accum := 0
		cursor := leftmost // BUG cursor is always starting on a wall, making the row passed 1 tile too wide. I bet the row always ends with a wall also.
		if width[y] == 0 {
			panic("0width row")
		}
		sc.MoveTo(leftmost)
		sc.GetRow(liWalls, wallRow[:width[y]])
		sc.GetRow(liTangent, tangentRow[:width[y]])
		sc.GetRow(liRids, ridRow[:width[y]])
		//rgWalls.GetRow(leftmost, wallRow[:width[y]+1]) //  BUG nuke these +1's
		//rgTangent.GetRow(leftmost, tangentRow[:width[y]+1])
		//rgRids.GetRow(leftmost, ridRow[:width[y]+1])
		for i := 0; i < width[y]; i++ {
			//rowMask.Mask[i] = false
			delta := int(tangentRow[i])
			wall := wallRow[i]
			accum += delta
			if accum > 2 || accum < -2 {
				fmt.Println(tangentRow)
				panic("tangent layer error")
			}
			if delta == 0 &&
				(accum == -2 || accum == 2) &&
				wall == 0 {
				rid := RoomId(ridRow[i])
				if rid != lastRid {
					existingRoom = r.w.Rooms[rid]
					lastRid = rid
				}
				// On an interior tile of this room, or a sub-room, or a super-room. Decide if it should be covered with paint.
				if rid == ROOMID_INVALID || rid == r.id {
					// This tile belongs to no room, or already belongs to the room being painted. Cover it.
					paint = true
				} else {
					if existingRoom == nil || r.ApproxArea < existingRoom.ApproxArea {
						// This tile belongs to a deleted room or a larger super-room, cover it.
						paint = true
					} else {
						// Tile belongs to a sub-room, don't cover it.
						paint = false
					}
				}
				for {
					// Paint until a wall is hit, then loop around and decide if painting should continue.
					rowMask.Append(paint)
					cursor, _, _ = cursor.Right()
					i++
					delta = int(tangentRow[i])
					wall = wallRow[i]
					accum += delta
					if accum > 2 || accum < -2 {
						fmt.Println(tangentRow)
						panic("tangent layer error")
					}
					if delta != 0 ||
						wall != 0 { // TODO checking only wall suffices?
						// Hit wall, break this run.
						rowMask.Append(false)
						break
					}
					if i == width[y] {
						panic("runaway room")
					}
				}
			} else {
				rowMask.Append(false)
			}
			cursor, _, _ = cursor.Right()
		}
		if f(rowMask, ridRow[:width[y]]) == false {
			break
		}
	}
}

// Debug -- check if a room is connected (very slow)
func (r *Room) checkConnected() {
	f := r.w.RoomIds.DeepSearch(game.TileId(r.id))
	if len(f) != r.Area {
		panic("Area is wrong")
	}
	if len(f) == 0 {
		return
	}
	c := len(f)
	for range r.w.RoomIds.Flood(f[0]) {
		c--
	}
	if c != 0 {
		// A room has become disconnected. this is an error.
		disconnectedRoom := layer.NewLayerFromSlice(f, game.TileId(r.id))
		panic(LayerError{
			Layer:   disconnectedRoom,
			Message: "room not connected",
		})
	}
}

func (r *Room) addDoorId(d DoorId) {
	length := len(r.DoorIds)
	i := sort.Search(length, func(i int) bool {
		return d <= r.DoorIds[i]
	})
	if i < length && r.DoorIds[i] == d {
		// already referencing this door
		return
	}
	// insert d in position i
	r.DoorIds = append(r.DoorIds, 0)
	copy(r.DoorIds[i+1:], r.DoorIds[i:])
	r.DoorIds[i] = d
}

func (r *Room) removeDoorId(d DoorId) bool {
	length := len(r.DoorIds)
	i := sort.Search(length, func(i int) bool {
		return d <= r.DoorIds[i]
	})
	if i < length && r.DoorIds[i] == d {
		copy(r.DoorIds[i:], r.DoorIds[i+1:])
		r.DoorIds = r.DoorIds[:length-1]
		return true
	}
	return false
}

type roomIdRemap struct {
	Old, New RoomId
}
