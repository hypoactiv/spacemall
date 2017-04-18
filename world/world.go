package world

import (
	"fmt"
	"jds/game"
	"jds/game/layer"
	"jds/runstat"
	"runtime/debug"
	"sort"
	"sync"
	"time"
)

type Operation int

const (
	OP_ADD = iota
	OP_DELETE
)

func (o Operation) String() string {
	switch o {
	case OP_ADD:
		return "Add"
	case OP_DELETE:
		return "Delete"
	default:
		panic("invalid operation")
	}
}

var LastOp = struct {
	Type Operation
	Loc  game.Location
}{}

type LayerError struct {
	Message string
	Layer   *layer.Layer
}

func (l LayerError) String() string {
	return l.Message
}

type workUnit struct {
	Actors []game.Actor
	X      int
	locked bool
	done   bool
}

// There are 2 workUnit buffers in World. One for this tick (WU_EXECUTE) and
// one for next tick (WU_BUFFER). These two slices swap each tick.
const (
	WU_BUFFER  = iota // New Actions for next tick go in slice World.workUnits[WU_BUFFER]
	WU_EXECUTE        // Actions executing this tick go in slice World.workUnits[WU_EXECUTE]
)

type World struct {
	nextRoomId              RoomId
	nextDoorId              DoorId
	nextEntityId            EntityId
	WallNodes               map[game.Location]*WallTreeNode
	Rooms                   map[RoomId]*Room
	Doors                   map[DoorId]*Door
	Walls, RoomIds, DoorIds *layer.Layer
	EntityIds               *layer.Layer
	// The number of wall tiles in a connected complex of rooms. The key
	// is the pointer to the wall tree root.
	complexSize map[*WallTreeNode]int
	ForcedFlags *layer.Layer // flags to help pathfinding determine turning points
	// A stack of RoomId changes to be done after an Add operation completes
	roomIdRemapStack  []roomIdRemap
	strict            int
	AddOps, DeleteOps int
	//
	sc       layer.StackCursor
	Entities map[EntityId]Entity
	// New thoughts (events to be scheduled) come in on this channel
	thinkHeap game.ThinkHeap
	workUnits [2][]workUnit
	//
	ticks        game.Ticks
	ActionCount  int
	customLayers map[string]*layer.Layer
	clMutex      sync.Mutex
}

const (
	STRICT_FSCK_EVERY_OP = 1 << iota
	STRICT_PARANOID
	STRICT_ALL = (1 << iota) - 1
)

const (
	wallIndex = iota
	flagIndex
	roomIndex
	entityIndex
)

func (w *World) CustomLayer(name string) (l *layer.Layer) {
	w.clMutex.Lock()
	l = w.customLayers[name]
	if l == nil {
		l = layer.NewLayer()
		w.customLayers[name] = l
	}
	w.clMutex.Unlock()
	return
}

// buffer a thought for next tick
func (w *World) bufferThoughts(ta *game.ThoughtAccumulator) {
	if ta == nil {
		return
	}
	bufferThoughtsInner := func(t []game.Thought) {
		if len(t) == 0 {
			return
		}
		// thoughts in t all have same X coordinate
		length := len(w.workUnits[WU_BUFFER])
		// workUnits must be sorted by X value (sorted by column) to allow
		// work to be split between goroutines later
		i := sort.Search(length, func(i int) bool {
			return t[0].BlockId.X <= w.workUnits[WU_BUFFER][i].X
		})
		if i == length || w.workUnits[WU_BUFFER][i].X != t[0].BlockId.X {
			// No workUnit for this column yet
			w.workUnits[WU_BUFFER] = append(w.workUnits[WU_BUFFER], workUnit{})
			copy(w.workUnits[WU_BUFFER][i+1:], w.workUnits[WU_BUFFER][i:])
			w.workUnits[WU_BUFFER][i] = workUnit{X: t[0].BlockId.X}
		}
		// Append this Thought's Action to the workUnit for this column
		for _, th := range t {
			w.workUnits[WU_BUFFER][i].Actors = append(w.workUnits[WU_BUFFER][i].Actors, th.Do)
		}
	}
	// put thoughts for later ticks on thinkHeap
	for _, v := range ta.LaterTicks {
		w.thinkHeap.Schedule(v)
	}
	// sort thoughts for next tick by X value and add to workUnits[WU_BUFFER]
	runStart := 0
	for i := range ta.NextTick {
		if i > 0 && ta.NextTick[i-1].BlockId.X != ta.NextTick[i].BlockId.X {
			bufferThoughtsInner(ta.NextTick[runStart:i])
			runStart = i
		}
	}
	bufferThoughtsInner(ta.NextTick[runStart:])
}

/*func (w *World) processNewThoughts() {
	//fmt.Printf("now processing thoughts buf:%p exe:%p\n", w.workUnits[WU_BUFFER], w.workUnits[WU_EXECUTE])
	defer w.wgProcessThoughts.Done()
	for t := range w.newThoughts {
		if t.At == w.ticks+1 {
			// schedule thoughts for next tick into WU_BUFFER workUnits immediately
			w.bufferThoughts(t)
		} else if t.At <= w.ticks {
			panic("tried to schedule thought for current or past tick")
		} else {
			// push later thoughts onto thinkHeap
			w.thinkHeap.Schedule(t)
		}
	}
	//fmt.Println("stopped processing thoughts")
}*/

func NewWorld(strictFlags int) *World {
	w := &World{
		WallNodes:    make(map[game.Location]*WallTreeNode),
		nextRoomId:   2,
		nextDoorId:   2,
		nextEntityId: 2,
		Rooms:        make(map[RoomId]*Room),
		complexSize:  make(map[*WallTreeNode]int),
		Doors:        make(map[DoorId]*Door),
		Entities:     make(map[EntityId]Entity),
		customLayers: make(map[string]*layer.Layer),
		strict:       strictFlags,
		DoorIds:      layer.NewLayer(),
		EntityIds:    layer.NewLayer(),
		ForcedFlags:  layer.NewLayer(),
		RoomIds:      layer.NewLayer(),
		Walls:        layer.NewLayer(),
		sc:           layer.NewStackCursor(game.Location{}),
	}
	if w.sc.Add(w.Walls) != wallIndex ||
		w.sc.Add(w.ForcedFlags) != flagIndex ||
		w.sc.Add(w.RoomIds) != roomIndex ||
		w.sc.Add(w.EntityIds) != entityIndex {
		panic("unexpected layer index")
	}
	return w
}

type WallTreeNode struct {
	// Location of the wall tile
	L game.Location
	// Pointer to parent
	P *WallTreeNode
	// Direction from here to parent
	D game.Direction
	// Pointers to neighbors
	N [4]*WallTreeNode
	// Pointer to tree root
	R *WallTreeNode
	// Distance to root node
	Depth int
	// If this wall closes a loop, store the RoomId of the created room, and the direction of the linking tile that closed the loop
	// TODO rename this to LinkingNodeParentDirections
	RoomIds map[RoomId]game.Direction
	// temporary for computing tangent
	t int
}

// Finds the first common ancestor between a and b
func commonAncestor(a, b *WallTreeNode) *WallTreeNode {
	if a.R != b.R {
		panic("a and b are not from the same tree")
	}
	if a.Depth < b.Depth {
		a, b = b, a
	}
	for a.Depth > b.Depth {
		a = a.P
	}
	for a != b {
		if a == nil || b == nil {
			panic("no common anscestor found")
		}
		a, b = a.P, b.P
	}
	return a
}

// Computes extreme left and right points of a loop for each row it occupies
func computeLeftAndRightMost(a, b, stop *WallTreeNode) (left []game.Location, width []int, maxWidth int) {
	if a.L != b.L {
		panic("a and b are not the same tile")
	}
	// Compute the height of a bounding box for the loop
	yMin, yMax := 0, 0
	xMin, xMax := 0, 0
	bound := func(n *WallTreeNode) {
		x := 0
		y := 0
		for {
			// (x,y) contains relative location of n
			if x < xMin {
				xMin = x
			} else if x > xMax {
				xMax = x
			}
			if y < yMin {
				yMin = y
			} else if y > yMax {
				yMax = y
			}
			if n == stop {
				break
			}
			// Update (x,y) to relative location of n.P
			if n.D == game.LEFT {
				x--
			} else if n.D == game.RIGHT {
				x++
			} else if n.D == game.DOWN {
				y++
			} else if n.D == game.UP {
				y--
			}
			// (x,y) contains relative location of n.P
			n = n.P
			// (x,y) contains relative location of n
		}
	}
	bound(a)
	bound(b)
	l := make([]int, yMax-yMin+1)
	r := make([]int, yMax-yMin+1)
	left = make([]game.Location, yMax-yMin+1)
	width = make([]int, yMax-yMin+1)
	right := make([]game.Location, yMax-yMin+1)
	// Measure each row of the room inside the bounding box
	measure := func(n *WallTreeNode) {
		x := 0
		y := 0
		for {
			/*
				x <= xMax
				x < xMax + 1
				x - xMax - 1 < 0
				The initial value of l[y-yMin] is 0, so this conditional is always
				true on the first iteration to reach each row, so left[y-yMin] is
				initialized to a valid location for all y.
			*/
			if x-xMax-1 < l[y-yMin] {
				l[y-yMin] = x - xMax - 1
				left[y-yMin] = n.L
			}
			// Ditto above
			if x-xMin+1 > r[y-yMin] {
				r[y-yMin] = x - xMin + 1
				right[y-yMin] = n.L
			}
			width[y-yMin] = (r[y-yMin] + xMin - 1) - (l[y-yMin] + xMax + 1)
			if width[y-yMin] > maxWidth {
				maxWidth = width[y-yMin]
			}
			if n == stop {
				break
			}
			// Update (x,y) to relative location of n.P
			if n.D == game.LEFT {
				x--
			} else if n.D == game.RIGHT {
				x++
			} else if n.D == game.DOWN {
				y++
			} else if n.D == game.UP {
				y--
			}
			n = n.P
		}
	}
	measure(a)
	measure(b)
	return
}

func (w *World) Fsck() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			panic(r)
			fmt.Println("continuing after fsck errors")
		}
	}()
	w.fsck()
}

func (w *World) fsck() {
	defer runstat.Record(time.Now(), "FsckWallTree")
	//n := w.WallNodes[l]
	//if n == nil {
	//		panic("location is not in a wall tree")
	//	}
	seenRids := make(map[RoomId]game.Location)
	// Check all nodes
	for l, nn := range w.WallNodes {
		// Check node location
		if nn.L != l {
			panic("location is wrong")
		}
		if w.RoomIds.Get(nn.L) != 0 {
			errorLayer := layer.NewLayer()
			errorLayer.Set(nn.L, 1)
			panic(LayerError{
				Layer:   errorLayer,
				Message: "wall with non-zero roomid under it",
			})
		}
		// All adjacent wall tiles should be in the same tree
		for _, nl := range nn.L.Neighbors() {
			if nnn := w.WallNodes[nl]; nnn != nil {
				if nnn.R != nn.R {
					panic("adjacent tile from different tree")
				}
			}
		}
		// Neighbor pointers
		if nn.P != nil && nn.P.N[3-nn.D] != nn {
			fmt.Println("consistency error", nn.L.String(), nn.P.L.String())
			panic("neighbor pointer consistency error")
		}
		// Check node's Room pointers
		for rid, d := range nn.RoomIds {
			if rid == ROOMID_INVALID {
				panic("node contains invalid room id")
			}
			r := w.Rooms[rid]
			if r == nil {
				w.dumpRooms()
				fmt.Println("rid=", rid)
				panic("node contains reference to deleted room")
			}
			// nn is the LNP for Room r
			if nn.L != r.LNPL() {
				panic("LNPL inconsistent")
			}
			if nn != r.LNP() {
				panic("LNP inconsistent")
			}
			if d != r.LNPD {
				panic("LNPD inconsistent")
			}
			if _, ok := seenRids[rid]; ok {
				fmt.Println("rid", rid, nn.L, seenRids[rid])
				panic("more than one LNP for rid")
			}
			seenRids[rid] = nn.L
		}
		// Check node depth
		depth := 0
		for nnn := nn; nnn != nnn.R; nnn = nnn.P {
			if nnn.Depth != nnn.P.Depth+1 {
				fmt.Println(nnn.Depth)
				panic("node depth incorrect1")
			}
			depth++
		}
		if depth != nn.Depth {
			fmt.Println("want", nn.Depth, "got", depth)
			panic("node depth incorrect")
		}
	}
	// Check all rooms
	// Map range traversal order is random, so sort keys first
	rids := make([]int, 0)
	for rid, r := range w.Rooms {
		if r.id != rid {
			panic("stored rid doesn't match key")
		}
		rids = append(rids, int(rid))
	}
	sort.Ints(rids)
	// check room structs
	for _, rid := range rids {
		r := w.Rooms[RoomId(rid)]
		n1 := r.LinkingNode()
		n2 := w.WallNodes[n1.L]
		if n2 == nil || n1.R != n2.R {
			fmt.Println("rid", rid)
			fmt.Println(n1, n2)
			fmt.Println(len(w.WallNodes), n1.L)
			layer := layer.NewLayer()
			layer.Set(n1.L, 1)
			panic(LayerError{
				Message: "linking node is from different tree",
				Layer:   layer,
			})
		}
		if d, ok := r.LNP().RoomIds[r.id]; ok {
			if d != r.LNPD {
				panic("r.LNP's direction inconsistent with r's")
			}
		} else {
			panic("room points to node, but node doesn't point to room")
		}
	}
	roomTiles := make(map[game.Location]bool)
	if w.strict&STRICT_PARANOID != 0 {
		for _, loc := range w.RoomIds.DeepSearchNonZero() {
			roomTiles[loc] = true
		}
	}
	for _, rid := range rids {
		rid := RoomId(rid)
		r := w.Rooms[RoomId(rid)]
		interiorArea := 0
		r.Interior(func(rm *game.RowMask, ridRow []game.TileId) bool {
			// sanity check
			if rm.Width() != len(ridRow) {
				panic("interior error")
			}
			for i := 0; i < rm.Width(); i++ {
				inside, skip := rm.Mask(i)
				if inside {
					interiorArea++
					if RoomId(ridRow[i]) != rid {
						fmt.Println("got", ridRow[i], "wanted", rid, "actual", w.RoomIds.Get(rm.Left))
						panic("interior error")
					}
					rm.Left, _, _ = rm.Left.Right()
				} else {
					i += skip - 1
					rm.Left, _, _ = rm.Left.Offset(skip, 0)
				}
			}
			return true
		})
		if interiorArea != r.Area {
			roomTiles := w.RoomIds.DeepSearch(game.TileId(rid))
			trueArea := len(roomTiles)
			panic(LayerError{
				Message: fmt.Sprintln("rid", rid, "interior area", interiorArea, "actual area", trueArea, "recorded area", r.Area),
				Layer:   layer.NewLayerFromSlice(roomTiles, game.TileId(rid)),
			})
		}
		nonempty, il := r.IsNonEmpty()
		if !nonempty && r.Area != 0 {
			panic("room is empty, but has positive area")
		}
		if w.strict&STRICT_PARANOID != 0 {
			// Exhaustively verify room area is correct and rooms are connected. very slow.
			connectedArea := 0
			if nonempty {
				for _, l := range w.RoomIds.Flood(il) {
					roomTiles[l] = false
					connectedArea++
					if connectedArea > 10000 {
						panic("room area limit hit")
					}
					if RoomId(w.RoomIds.Get(l)) != rid {
						panic("room id's not consistent")
					}
				}
			}
			if connectedArea != r.Area {
				roomTiles := w.RoomIds.DeepSearch(game.TileId(rid))
				trueArea := len(roomTiles)
				panic(LayerError{
					Message: fmt.Sprintln("rid", rid, "computed area", connectedArea, "actual area", trueArea, "recorded area", r.Area),
					Layer:   layer.NewLayerFromSlice(roomTiles, game.TileId(rid)),
				})
			}
		}
		// Check Door references
		for _, did := range r.DoorIds {
			d := w.Doors[did]
			for _, rid := range d.R {
				if rid == r.id {
					goto okay
				}
			}
			panic("Missing Door->Room reference. Room contains stale DoorIds entry?")
		okay:
		}
	}
	// verify that every room tile was visited. this is vacuous if STRICT_PARANOID is not set (roomTiles uninitialized)
	for k, v := range roomTiles {
		if v == true {
			panic(fmt.Sprintln(k, "belongs to non-existent room", w.RoomIds.Get(k)))
		}
	}
	// Check all doors
	for _, v := range w.Doors {
		v.fsck()
	}
}

// Sets the RoomId of all Interior tiles of r to 0, and updates all connected
// doors
func (r *Room) clear(m game.ModMap) {
	r.Interior(func(rm *game.RowMask, ridRow []game.TileId) bool {
		m.AddRowMask(rm)
		r.w.RoomIds.SetRowMask(rm, 0, m)
		return true
	})
	for _, did := range r.DoorIds {
		door := r.w.Doors[did]
		door.updateRids()
	}
	return
}

func (w *World) changeRoomId(start game.Location, old, new RoomId) {
	if new == 0 {
		panic("can't assign roomId 0")
	}
	if old == 0 {
		panic("can't replace roomId 0")
	}
	if RoomId(w.RoomIds.Get(start)) != old {
		panic("invalid start location room id")
	}
	if _, ok := w.Rooms[new]; ok {
		l := layer.NewLayerFromSlice(w.RoomIds.DeepSearch(game.TileId(new)), game.TileId(new))
		l.SetSlice(w.RoomIds.DeepSearch(game.TileId(old)), game.TileId(old))
		panic(LayerError{
			Layer:   l,
			Message: fmt.Sprintf("New RoomID already in use. Old:%d New:%d", old, new),
		})
	}
	r := w.Rooms[old]
	r.Interior(func(rm *game.RowMask, rid []game.TileId) bool {
		for i := 0; i < rm.Width(); i++ {
			l, _ := rm.Mask(i)
			if l {
				w.RoomIds.Set(rm.Left, game.TileId(new))
			}
			rm.Left, _, _ = rm.Left.Right()
		}
		return true
	})
	r.id = new
	w.Rooms[r.id] = r
	// update linking node's direction map
	r.LNP().RoomIds[r.id] = r.LNP().RoomIds[old]
	// Update doors
	for _, did := range r.DoorIds {
		door := w.Doors[did]
		door.updateRids()
	}
	// Cleanup old pointers
	delete(r.LNP().RoomIds, old)
	delete(w.Rooms, old)
}

func (w *World) DeleteFromWallTree(loc game.Location) (m game.ModMap) {
	m = game.NewModMap()
	LastOp.Type = OP_DELETE
	LastOp.Loc = loc
	// If there is a door here, delete it first
	if did := DoorId(w.DoorIds.Get(loc)); did != 0 {
		door := w.Doors[did]
		door.Delete(m)
	}
	w.deleteFromWallTree(loc, m)
	w.DeleteOps++
	w.runRoomIdChanges()
	w.updateForcedFlags(loc)
	if w.strict&STRICT_FSCK_EVERY_OP != 0 {
		w.Fsck()
	}
	return m
}

func (w *World) deleteFromWallTree(locationToDelete game.Location, m game.ModMap) {
	defer runstat.Record(time.Now(), "DeleteFromWallTree")
	node := w.WallNodes[locationToDelete] //Delete this node
	if node == nil {
		return
	}
	m.AddLocation(locationToDelete)
	// EXP clear neighboring rooms
	nbd_rooms := make(map[RoomId]*WallTreeNode)
	largestRoom := RoomId(ROOMID_INVALID)
	largestArea := 0
	for _, l := range locationToDelete.Neighborhood() {
		if rid := RoomId(w.RoomIds.Get(l)); w.Walls.Get(l) == 0 && rid != 0 {
			neighborRoom := w.Rooms[rid]
			if largestRoom == ROOMID_INVALID || neighborRoom.Area > largestArea {
				largestRoom = rid
				largestArea = neighborRoom.Area
			}
			nbd_rooms[rid] = neighborRoom.LNP()
			neighborRoom.clear(m)
		}
	}
	// Clear parent's neighbor pointer if it exists
	if node.P != nil {
		node.P.N[3-node.D] = nil
	}
	// Clear node at l and recursively clear all children
	children := w.deleteWallTree(node, 0)
	// Recursively re-add all children of l
	for i := len(children) - 1; i >= 1; i-- {
		c := children[i]
		w.Walls.Set(c, 1)
		w.addToWallTree(c, m)
	}
	// EXP recolor merged rooms that still exist
	for rid := range nbd_rooms {
		room := w.Rooms[rid]
		if room != nil {
			room.recolor(m)
		}
	}
	// After deleting a wall, some rooms may be merged.
	// It is desirable to have the RoomId of the merged room be that of the
	// largest (by area) constituent of the merge. If the RoomId at the deleted
	// location is not that of the largest room, schedule it to be changed.
	newRid := RoomId(w.RoomIds.Get(locationToDelete))
	if largestRoom != ROOMID_INVALID && newRid != largestRoom && newRid != 0 {
		w.roomIdRemapStack = append(w.roomIdRemapStack, roomIdRemap{Old: newRid, New: largestRoom})
	}
	// Discard the neighborhood rooms, except the new one, if it survived intact
	for rid, n := range nbd_rooms {
		if rid != newRid {
			if _, ok := n.RoomIds[rid]; ok {
				panic("this is still needed")
			}
			if _, ok := w.Rooms[rid]; ok {
				panic("this is still needed")
			}
			delete(n.RoomIds, rid)
			delete(w.Rooms, rid)
		}
	}
	return
}

func (w *World) SetWall(l game.Location) game.ModMap {
	if !w.CanSetWall(l) {
		return nil
	}
	w.Walls.Set(l, 1)
	return w.AddToWallTree(l)
}

func (w *World) CanSetWall(l game.Location) bool {
	if w.DoorIds.Get(l) != 0 {
		// Wall would block a door
		return false
	}
	w.sc.MoveTo(l)
	wallLocal := w.sc.Look(wallIndex)
	switch {
	case wallLocal[game.RIGHT] != 0 && wallLocal[game.RIGHTUP] != 0 && wallLocal[game.UP] != 0:
		return false
	case wallLocal[game.UP] != 0 && wallLocal[game.LEFTUP] != 0 && wallLocal[game.LEFT] != 0:
		return false
	case wallLocal[game.LEFT] != 0 && wallLocal[game.LEFTDOWN] != 0 && wallLocal[game.DOWN] != 0:
		return false
	case wallLocal[game.DOWN] != 0 && wallLocal[game.RIGHTDOWN] != 0 && wallLocal[game.RIGHT] != 0:
		return false
	}
	return true
}

func (w *World) AddToWallTree(l game.Location) (m game.ModMap) {
	if !w.CanSetWall(l) {
		panic("tried to add an invalid wall")
	}
	LastOp.Type = OP_ADD
	LastOp.Loc = l
	m = game.NewModMap()
	m.AddLocation(l)
	w.addToWallTree(l, m)
	w.AddOps++
	w.runRoomIdChanges()
	w.ForcedFlags.Set(l, game.TileId(0xff)) // walls have all forced flags set, so pathfinding jumps in every direction will stop at walls
	w.updateForcedFlags(l)
	if w.strict&STRICT_FSCK_EVERY_OP != 0 {
		w.Fsck()
	}
	return m
}

// Update neighbor forced flags, call after adding or removing wall at l
func (w *World) updateForcedFlags(l game.Location) {
	w.sc.MoveTo(l)
	wallLocal := w.sc.Look(wallIndex)
	for d, nl := range l.Neighborhood() {
		d := game.Direction(d)
		if wallLocal[d] != 0 {
			// leave walls at 0xff as set above
			continue
		}
		w.sc.MoveTo(nl)
		wallLocalN := w.sc.Look(wallIndex)
		w.sc.Set(flagIndex, game.TileId(ForcedFlags(wallLocalN)))
	}
}

func (w *World) runRoomIdChanges() {
	for _, r := range w.roomIdRemapStack {
		room := w.Rooms[r.Old]
		if room == nil {
			// This room no longer exists, so no recolor possible
			continue
		}
		if w.Rooms[r.New] != nil {
			// Some other room remapped to this one, recolor not possible
			continue
		}
		nonempty, il := room.IsNonEmpty()
		if !nonempty {
			panic("empty room not cleaned up")
		}
		w.changeRoomId(il, r.Old, r.New)
		// Update door references
		for _, did := range room.DoorIds {
			door := w.Doors[did]
			door.updateRids()
		}
	}
	w.roomIdRemapStack = nil
}

func (w *World) addToWallTree(locationToAdd game.Location, m game.ModMap) {
	//defer runstat.Record(time.Now(), "AddToWallTree")
	w.sc.MoveTo(locationToAdd)
	if w.sc.Get(wallIndex) != 1 {
		panic("not a wall")
	}
	// If this location is in a room, decrease its area
	if rid := RoomId(w.sc.Get(roomIndex)); rid != 0 {
		w.Rooms[rid].Area--
		w.RoomIds.Set(locationToAdd, 0)
	}
	if w.WallNodes[locationToAdd] != nil {
		// already part of a tree
		return
	}
	type pseudonode struct { // TODO rename to directed node
		N *WallTreeNode
		D game.Direction
	}
	neighbors := make([]pseudonode, 0, 4)
	largestSize := -1
	largest := 0
	wallLocal := w.sc.Look(wallIndex)
	for d, ln := range locationToAdd.Neighbors() {
		d := game.Direction(d)
		if wallLocal[d] != 1 {
			continue
		}
		if nn := w.WallNodes[ln]; nn != nil {
			if cs := w.complexSize[nn.R]; cs > largestSize {
				largest = len(neighbors)
				largestSize = cs
			}
			neighbors = append(neighbors, pseudonode{nn, game.Direction(d)})
		}
	}
	if len(neighbors) == 0 {
		// If l has 0 neighbors, make a new wall tree with l as its root.
		// No new loops possible
		n := getNode()
		*n = WallTreeNode{
			L:     locationToAdd,
			Depth: 0,
		}
		n.R = n
		w.WallNodes[locationToAdd] = n
		w.complexSize[n] = 1
	} else if len(neighbors) == 1 {
		// If l has 1 neighbor, add l to the wall tree of its neighbor
		// No new loops possible
		ln := &neighbors[0]
		// Get neighbor's wall tree node
		n := ln.N
		if n == nil {
			panic("neighbor wall tree not initialized at " + ln.N.L.String())
		}
		// Attach as child of neighbor
		n.N[ln.D.Reverse()] = getNode()
		*n.N[3-ln.D] = WallTreeNode{
			L:     locationToAdd,
			P:     n,
			Depth: n.Depth + 1,
			D:     ln.D,
			R:     n.R,
		}
		w.WallNodes[locationToAdd] = n.N[3-ln.D]
		w.complexSize[n.R]++
	} else {
		// Merge 2 wall trees
		//
		// If locationToAdd has more than 1 neighbor, find neighbor with largest
		// complexSize, and destroy the wall trees for all other neighbors. Then
		// add locationToAdd to the wall tree of largest neighbor, and recusively
		// add nodes of the destroyed trees as descendents of locationToAdd.
		largestPseudonode := neighbors[largest]
		largestRoot := largestPseudonode.N.R
		for _, neighbor := range neighbors {
			nn := neighbor.N
			if nn.R != largestRoot {
				if _, ok := w.WallNodes[neighbor.N.L]; !ok {
					// already deleted
					continue
				}
				w.deleteWallTree(nn.R, 2)
			}
		}
		// Add new node as a child of ln
		n := getNode()
		*n = WallTreeNode{
			L: locationToAdd,
			P: largestPseudonode.N,
			R: largestRoot,
			D: largestPseudonode.D,
		}
		n.Depth = n.P.Depth + 1
		// Link to parent
		n.P.N[n.D.Reverse()] = n
		q := make([]*WallTreeNode, 1)
		q[0] = n
		w.sc.MoveTo(n.L)
		w.sc.Set(wallIndex, 1)
		for len(q) > 0 {
			var n *WallTreeNode
			n, q = q[0], q[1:]
			// Detect loops by reaching a block we've visited before
			if linking := w.WallNodes[n.L]; linking != nil {
				// This tile already has a WallTreeNode associated with it, so a loop
				// has been found. The duplicate WallTreeNode is called a
				// "linking node," as it links two paths back to a common anscestor to
				// form a loop, which is then made into the perimeter of a room. To
				// save this discovery, a marker is set on the parent of the linking
				// node which indicates which direction the linking node was
				// discovered, and the RoomId of the resultant room. The linking node
				// is then discarded, to be reconstructed by the information stored in
				// its parent when needed.
				if linking.R != n.R {
					// Adjacent tiles must belong to the same WallTree root, so this
					// cannot happen.
					panic("linking node is from a different tree")
				}
				// Unlink from parent to discard linking node
				n.P.N[3-n.D] = nil
				cacheNode(n)
				w.newRoom(n.P, n.D, m) // Make new room with linking node n
				continue
			}
			w.WallNodes[n.L] = n
			w.complexSize[n.R]++
			// Add neighbors of n, breadth first, to the tree
			neighbors := n.L.Neighbors()
			for i, nb := range neighbors {
				// If n has a parent and are moving in the parent direction...
				if n.P != nil && game.Direction(i) == n.D {
					// ...don't revisit parent
					continue
				}
				w.sc.MoveTo(nb)
				if nn := w.WallNodes[nb]; w.sc.Get(wallIndex) == 2 || (nn != nil && nn.R == n.R) {
					w.sc.Set(wallIndex, 1)
					n.N[i] = getNode()
					*(n.N[i]) = WallTreeNode{
						L: nb,
						P: n,
						// since i is the direction from n to nb, 3-i
						// is the direction from nb to n
						D:     3 - game.Direction(i),
						R:     n.R,
						Depth: n.Depth + 1,
					}
					q = append(q, n.N[i])
				}
			}
		}
	}
	return
}

// Deletes node n and all subnodes, cleaning up deleted Rooms as it goes
// Returns a list of deleted node locations
//
// value 'v' is written to Walls layer at all altered wall tiles
func (w *World) deleteWallTree(n *WallTreeNode, v game.TileId) (children []game.Location) {
	// Unlink from parent
	if n.P != nil {
		n.P.N[3-n.D] = nil
	}
	q := make([]*WallTreeNode, 1, 20)
	q[0] = n
	for len(q) > 0 {
		n, q = q[0], q[1:]
		w.sc.MoveTo(n.L)
		wallLocal := w.sc.Look(wallIndex)
		// check neighbors to see if n is a linking location
		// If so, remove the reference in the neighbor and delete the room
		for d, nl := range n.L.Neighbors() {
			if wallLocal[d] != 1 {
				continue
			}
			if nn := w.WallNodes[nl]; nn != nil {
				for rid, ld := range nn.RoomIds {
					if game.Direction(d) == ld {
						delete(nn.RoomIds, rid)
						delete(w.Rooms, rid)
						// TODO trigger notifications of potential RoomId change (rid->?)
						// to connected Doors, etc. use some map to track modified/deleted rid's?
					}
				}
			}
		}
		// delete all rooms for which n is the linking node parent
		for rid := range n.RoomIds {
			delete(w.Rooms, rid)
		}
		children = append(children, n.L)
		delete(w.WallNodes, n.L)
		w.sc.MoveTo(n.L)
		w.sc.Set(wallIndex, v)
		cacheNode(n)
		for _, nn := range n.N {
			if nn != nil {
				q = append(q, nn)
			}
		}
	}
	if w.strict&STRICT_PARANOID != 0 {
		// Sanity check -- make sure that n appears nowhere
		for _, nn := range w.WallNodes {
			if nn == n {
				panic("n survived")
			}
			if nn.P == n {
				panic("n survived0")
			}
			if nn.R == n {
				panic("n survived1")
			}
			for _, nnn := range nn.N {
				if nnn == n {
					panic("n survived2")
				}
			}
		}
		for _, r := range w.Rooms {
			if r.LNP() == n {
				panic("n survived3")
			}
		}
	}
	return
}

func (w *World) newRoom(lnp *WallTreeNode, lnpd game.Direction, m game.ModMap) *Room {
	r := getRoom()
	*r = Room{
		w:    w,
		id:   w.nextRoomId,
		LNPD: lnpd,
	}
	r.LNL = lnp.L.JustStep(lnpd.Reverse())
	w.nextRoomId++
	if lnp.RoomIds == nil {
		lnp.RoomIds = make(map[RoomId]game.Direction)
	}
	lnp.RoomIds[r.id] = lnpd
	w.Rooms[r.id] = r
	r.recolor(m)
	return r
}

func (w *World) dumpRooms() {
	var rids []int
	fmt.Println("Room dump")
	for rid, r := range w.Rooms {
		if r.id != rid {
			panic("stored rid doesn't match key")
		}
		rids = append(rids, int(rid))
	}
	sort.Ints(rids)
	for _, rid := range rids {
		r := w.Rooms[RoomId(rid)]
		fmt.Println("rid=", rid)
		fmt.Println("linking node parent location=", r.LNP)
		fmt.Println("*************************")
	}
}

func (w *World) checkRootContinuty(n *WallTreeNode) {
	fail := false
	for _, nl := range n.L.Neighbors() {
		if nn := w.WallNodes[nl]; nn != nil {
			if nn.R != n.R {
				fmt.Println(nn.L, n.L, "are different")
				fail = true
			}
		}
	}
	if fail {
		debug.PrintStack()
		fmt.Printf("%s %p\n", n.L, n.R)
		for _, nl := range n.L.Neighbors() {
			if nn := w.WallNodes[nl]; nn != nil {
				fmt.Printf("%s %p\n", nn.L, nn.R)
			}
		}
		fmt.Println("root ptr not continuous")
	}
}

// Computes the y (vertical) component of a vector tangent to the curve
// bounding a room, returning the result in tangentLayer. A value of 2 or -2
// indicates vertical tangent. This is used to determine tiles interior and
// exterior to the curve in Room.Interior(...)
//
// The approximate area inside the curve is computed via Stokes' theorem and
// returned in approxArea. This is approximate area because it includes the
// bounding curve blocks (walls) on the north and west faces of the curve.
// Rooms with smaller approxArea are painted over rooms with larger
// approxArea
func computeTangent(a, b, stop *WallTreeNode) (tangentLayer *layer.Layer, approxArea int) {
	clear := func(n *WallTreeNode) {
		for {
			n.t = 0
			if n == stop {
				break
			}
			n = n.P
		}
	}
	clear(a)
	clear(b)
	traverse := func(n *WallTreeNode, direction int) {
		x := 0
		for n != stop {
			if n.D == game.LEFT {
				x--
			} else if n.D == game.RIGHT {
				x++
			} else if n.D == game.UP {
				n.t += direction
				n.P.t += direction
				approxArea += x * direction
			} else if n.D == game.DOWN {
				n.t += -direction
				n.P.t += -direction
				approxArea -= x * direction
			}
			n = n.P
		}
	}
	traverse(a, 1)
	traverse(b, -1)
	tangentLayer = layer.NewLayer()
	collect := func(n *WallTreeNode) {
		sc := layer.NewStackCursor(n.L)
		ti := sc.Add(tangentLayer)
		for {
			sc.Set(ti, game.TileId(n.t))
			if n == stop {
				break
			}
			sc.Step(n.D)
			n = n.P
		}
	}
	collect(a)
	collect(b)
	tangentLayer.Set(a.L, game.TileId(a.t+b.t)) // This tangent value gets split between 2 nodes
	if approxArea < 0 {
		approxArea = -approxArea
	}
	return
}

// Draws a box of wall tiles
func (w *World) DrawBox(ul, lr game.Location) {
	for loc := range game.Box(ul, lr) {
		w.SetWall(loc)
	}
}

// Draws a line of wall tiles
func (w *World) DrawLine(a, b game.Location) {
	for loc := range game.Line(a, b) {
		w.SetWall(loc)
	}
}

// If forced directions must be considered while moving in
// direction d, then (f & (1 << d)) != 0.
func ForcedFlags(wallLocal [8]game.TileId) (f uint8) {
	if (wallLocal[game.RIGHTUP] == 0 && wallLocal[game.UP] != 0) ||
		(wallLocal[game.RIGHTDOWN] == 0 && wallLocal[game.DOWN] != 0) {
		f |= (1 << game.RIGHT)
	}
	if (wallLocal[game.LEFTUP] == 0 && wallLocal[game.UP] != 0) ||
		(wallLocal[game.LEFTDOWN] == 0 && wallLocal[game.DOWN] != 0) {
		f |= (1 << game.LEFT)
	}
	if (wallLocal[game.LEFTUP] == 0 && wallLocal[game.LEFT] != 0) ||
		(wallLocal[game.RIGHTUP] == 0 && wallLocal[game.RIGHT] != 0) {
		f |= (1 << game.UP)
	}
	if (wallLocal[game.LEFTDOWN] == 0 && wallLocal[game.LEFT] != 0) ||
		(wallLocal[game.RIGHTDOWN] == 0 && wallLocal[game.RIGHT] != 0) {
		f |= (1 << game.DOWN)
	}
	if (wallLocal[game.LEFTUP] == 0 && wallLocal[game.LEFT] != 0) ||
		(wallLocal[game.RIGHTDOWN] == 0 && wallLocal[game.DOWN] != 0) {
		f |= (1 << game.RIGHTUP)
	}
	if (wallLocal[game.RIGHTUP] == 0 && wallLocal[game.RIGHT] != 0) ||
		(wallLocal[game.LEFTDOWN] == 0 && wallLocal[game.DOWN] != 0) {
		f |= (1 << game.LEFTUP)
	}
	if (wallLocal[game.LEFTDOWN] == 0 && wallLocal[game.LEFT] != 0) ||
		(wallLocal[game.RIGHTUP] == 0 && wallLocal[game.UP] != 0) {
		f |= (1 << game.RIGHTDOWN)
	}
	if (wallLocal[game.RIGHTDOWN] == 0 && wallLocal[game.RIGHT] != 0) ||
		(wallLocal[game.LEFTUP] == 0 && wallLocal[game.UP] != 0) {
		f |= (1 << game.LEFTDOWN)
	}
	return
}

// Spawn Entity 'e' into World 'w'. Returns the EntityId assigned to 'e'
// and call's e's Spawned event.
func (w *World) Spawn(e Entity) EntityId {
	l := e.Location()
	sc := layer.NewStackCursor(l)
	sc.Add(w.EntityIds) // Layer index 0
	sc.Add(w.Walls)     // Layer index 1
	if otherEid := EntityId(sc.Get(0)); otherEid != ENTITYID_INVALID {
		// An entity is already there
		return ENTITYID_INVALID
	}
	id := w.nextEntityId
	sc.Set(0, game.TileId(id))
	w.nextEntityId++
	w.Entities[id] = e
	taTmp := game.AllocateTA(w.ticks + 1)
	taTmp.Add(
		w.ticks+1,
		&spawnActor{e, id, w, &sc},
		l.BlockId,
	)
	w.bufferThoughts(taTmp)
	game.ReleaseTA(taTmp)
	return id
}

type spawnActor struct {
	e  Entity
	id EntityId
	w  *World
	sc *layer.StackCursor
}

func (sa *spawnActor) Act(ta *game.ThoughtAccumulator) {
	sa.e.Spawned(ta, sa.id, sa.w, sa.sc)
}

// sc must be a stack cursor at the entity's current location, with w.EntityIds
// as layer index 0, and e.Walls as layer index 1 (the same one passed during the
// Spawned event.)
//
// upon return, sc's cursor will be at the entity's new location
//
// true is returned if and only if the step is taken
func (w *World) StepEntity(eid EntityId, e Entity, sc *layer.StackCursor, d game.Direction) (game.Location, bool) {
	if d == game.NONE {
		return sc.Cursor(), true
	}
	// Collide with wall?
	if sc.DirectedGet(1, d) != 0 {
		e.HitWall(d)
		return sc.Cursor(), false
	}
	// Collide with other entity?
	if otherEid := EntityId(sc.DirectedGet(0, d)); otherEid != ENTITYID_INVALID {
		e.Touched(otherEid, d)
		return sc.Cursor(), false
	}
	// Move okay
	sc.Set(0, ENTITYID_INVALID)
	sc.Step(d)
	sc.Set(0, game.TileId(eid))
	return sc.Cursor(), true
}

func (w *World) Now() game.Ticks {
	return w.ticks
}

/*func (w *World) ScheduleThought(at game.Ticks, do game.Action, bid game.BlockId) {
	w.newThoughts <- game.Thought{
		At:      at,
		Do:      do,
		BlockId: bid,
	}
	//w.thinkHeap.Schedule(at, do, bid)
}*/

func (w *World) Think() {
	// increment time
	//w.ticks++
	// check thinkHeap for thoughts scheduled for w.ticks and buffer them
	// thinkHeap sorted by ticks
	taTmp := game.AllocateTA(w.ticks + 1)
	for w.thinkHeap.Len() > 0 {
		if w.thinkHeap.PeekTicks() > w.ticks {
			// no more thoughts for w.ticks
			break
		}
		taTmp.AddThought(w.thinkHeap.Next())
	}
	w.bufferThoughts(taTmp)
	game.ReleaseTA(taTmp)
	// Swap buffer and execute work units
	w.workUnits[WU_BUFFER], w.workUnits[WU_EXECUTE] = w.workUnits[WU_EXECUTE], w.workUnits[WU_BUFFER]
	// w.workUnits[WU_EXECUTE] now contains Actions to be performed during tick
	// w.ticks, sorted by BlockId X value of their actor
	wuExe := w.workUnits[WU_EXECUTE]
	wuLen := len(wuExe)
	type workerCommand struct {
		WU                  []workUnit
		TA                  *game.ThoughtAccumulator
		lockStart, lockStop int
	}
	type workerResponse struct {
		Commands chan<- workerCommand
		TA       *game.ThoughtAccumulator
	}
	wg := sync.WaitGroup{}
	responseChannel := make(chan workerResponse, 2)
	worker := func() {
		defer wg.Done()
		cc := make(chan workerCommand, 2)
		responseChannel <- workerResponse{
			Commands: cc,
		}
		for c := range cc {
			for _, wu := range c.WU {
				for _, actor := range wu.Actors {
					actor.Act(c.TA)
				}
			}
			for i := c.lockStart; i < c.lockStop; i++ {
				wuExe[i].locked = false
			}
			responseChannel <- workerResponse{
				Commands: cc,
				TA:       c.TA,
			}
		}
	}

	//fmt.Println("executing", len(w.workUnits[WU_EXECUTE]), "work units")
	if len(w.workUnits[WU_EXECUTE]) == 0 {
		// nothing to do
		//fmt.Println("early bailout")
		return
	}
	numWorkers := 8
	wg.Add(numWorkers)
	go func() {
		// close response channel once all workers have quit
		wg.Wait()
		close(responseChannel)
	}()
	//fmt.Printf("starting thinkers %p\n", &w.workUnits[WU_EXECUTE][0])
	for i := 0; i < numWorkers; i++ {
		//fmt.Println("go")
		go worker()
	}
	processing := true
processLoop:
	for processing {
		processing = false
		// Read a worker response and buffer its new thoughts stored in a ThoughtAccumulator
		resp := <-responseChannel
		for k := range wuExe {
			if wuExe[k].done == true {
				continue
			}
			if len(wuExe[k].Actors) == 0 {
				wuExe[k].done = true
				continue
			}
			// still more workUnits to process
			processing = true
			if wuExe[k].locked ||
				(k > 0 && wuExe[k-1].locked) ||
				(k < wuLen-1 && wuExe[k+1].locked) {
				// column k or a neighbor is locked, can't process
				continue
			}
			// column 'k' can be processed. lock it and its neighbors
			actionCount := len(wuExe[k].Actors)
			exeStart := k
			lockStart := k
			lockStop := k
			if k > 0 {
				lockStart = k - 1
			}
			// grab more columns to the right if possible
			for k < wuLen-2 && wuExe[k+1].done == false && wuExe[k+2].locked == false && actionCount < 2000 {
				k++
				actionCount += len(wuExe[k].Actors)
			}
			exeStop := k + 1
			if exeStop == wuLen-1 && wuExe[wuLen-1].done == false {
				actionCount += len(wuExe[wuLen-1].Actors)
				exeStop = wuLen
			}
			lockStop = exeStop + 1
			if lockStop > wuLen {
				lockStop = wuLen
			}
			for i := lockStart; i < lockStop; i++ {
				wuExe[i].locked = true
			}
			for i := exeStart; i < exeStop; i++ {
				wuExe[i].done = true
			}
			// Send workUnit to worker
			//fmt.Println(lockStart, exeStart, exeStop, lockStop)
			resp.Commands <- workerCommand{
				WU:        wuExe[exeStart:exeStop],
				TA:        game.AllocateTA(w.ticks + 1),
				lockStart: lockStart,
				lockStop:  lockStop,
			}
			w.ActionCount += actionCount
			wg.Add(1)
			go worker()
			w.bufferThoughts(resp.TA)
			game.ReleaseTA(resp.TA)
			continue processLoop
		}
		w.bufferThoughts(resp.TA)
		game.ReleaseTA(resp.TA)
		// no work for worker, shut it down
		close(resp.Commands)
	}
	// shut down remaining workers
	for resp := range responseChannel {
		w.bufferThoughts(resp.TA)
		game.ReleaseTA(resp.TA)
		close(resp.Commands)
	}
	// clear WU_EXECUTE for later reuse as WU_BUFFER
	for k := range w.workUnits[WU_EXECUTE] {
		v := &w.workUnits[WU_EXECUTE][k]
		v.Actors = v.Actors[:0]
		v.done = false
		v.locked = false
	}
	w.ticks++
}
