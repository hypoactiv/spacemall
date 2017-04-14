// Pathfinding (A*)

package path

import (
	"container/heap"
	"fmt"
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"jds/runstat"
	"sync"
	"time"
)

// Layer indices in the stack
const (
	wallIndex = iota
	gScoreIndex
	flagIndex
	openIndex
	closedIndex
)

type weightedWalker struct {
	sc      layer.StackCursor
	W       int
	D       game.Direction // incoming direction
	SegNode int            //*routeSegTreeNode
}

var pool sync.Pool

func init() {
	pool.New = func() interface{} {
		return new(weightedWalker)
	}
}

func (w *weightedWalker) hasForced() bool {
	if v := w.sc.Get(flagIndex); v&(1<<uint8(w.D)) != 0 {
		return true
	}
	return false
}

// Returns true if reached jump point, false if hit wall
func (w *weightedWalker) jump(finish game.Location) (bool, int) {
	jumpScanFast := func(d game.Direction) (bool, int) {
		scanDist := w.sc.ScanBit(flagIndex, d, -1, uint(d)) // the lesser of (distance to nearest tile with forced directions in direction d), or (the distance to nearest wall in direction d)
		dx, dy := w.sc.Cursor().Distance(finish)
		if dx == 0 || dy == 0 {
			// in same row or column as goal
			// will the goal be reached before hitting the wall or jump point?
			switch d {
			case game.RIGHT:
				if dy == 0 && dx >= 0 && int(dx) < scanDist {
					// will reach goal before wall or jump point
					return true, int(dx)
				}
			case game.UP:
				if dx == 0 && dy <= 0 && -int(dy) < scanDist {
					return true, -int(dy)
				}
			case game.DOWN:
				if dx == 0 && dy >= 0 && int(dy) < scanDist {
					return true, int(dy)
				}
			case game.LEFT:
				if dy == 0 && dx <= 0 && -int(dx) < scanDist {
					return true, -int(dx)
				}
				// will hit wall or jump point before goal
			}
		}
		if w.sc.FarStepGet(wallIndex, d, scanDist) != 0 {
			// there is a wall scanDist tiles away in direction d, return false
			// to indicate dead end
			return false, scanDist
		}
		// There is a jump point scanDist tiles away in direction d, return true
		// to indicate branches to be explored
		return true, scanDist
	}
	justJumpScanFast := func(d game.Direction) (jumpPoint bool) {
		jumpPoint, _ = jumpScanFast(d)
		return
	}
	i := 0
	var jumpPoint bool
	var jumpDist int
	if w.D < 4 {
		w.sc.Step(w.D)
		jumpPoint, jumpDist = jumpScanFast(w.D)
		if jumpPoint {
			w.sc.FarStep(w.D, jumpDist)
		}
		//w.sc.Step(w.D.Reverse())
		return jumpPoint, jumpDist + 1
	}
	for {
		i++
		if i > 1000 {
			panic("runaway")
		}
		w.sc.Step(w.D)
		if w.sc.Cursor() == finish {
			// reached goal
			if w.D < 4 && (i != jumpDist+1 || !jumpPoint) {
				fmt.Println(i, jumpDist, jumpPoint)
				panic("incon")
			}
			return true, i
		}
		if w.sc.Get(flagIndex)&(1<<uint8(w.D)) != 0 { //w.hasForced() { // also returns true for walls
			if w.sc.Get(wallIndex) != 0 {
				// hit wall
				return false, i
			}
			// jump point -- might change direction here
			return true, i
		}
		if w.D >= 4 {
			// moving diagonally
			switch w.D {
			case game.RIGHTUP:
				if justJumpScanFast(game.RIGHT) || justJumpScanFast(game.UP) {
					return true, i
				}
			case game.RIGHTDOWN:
				if justJumpScanFast(game.RIGHT) || justJumpScanFast(game.DOWN) {
					return true, i
				}
			case game.LEFTUP:
				if justJumpScanFast(game.LEFT) || justJumpScanFast(game.UP) {
					return true, i
				}
			case game.LEFTDOWN:
				if justJumpScanFast(game.LEFT) || justJumpScanFast(game.DOWN) {
					return true, i
				}
			}
		}
	}
}

func allocate() (w *weightedWalker) {
	nAlloc++
	w = pool.Get().(*weightedWalker)
	return w
}

func releaseAll(w *[]*weightedWalker) {
	nRelease += len(*w)
	for _, ww := range *w {
		pool.Put(ww)
	}
	*w = nil
}

func release(w **weightedWalker) {
	nRelease++
	pool.Put(*w)
	*w = nil
}

// Warp w to ww
func (w *weightedWalker) Warp(ww *weightedWalker) {
	layer.MoveStackCursor(&w.sc, &ww.sc)
}

type walkerHeap struct {
	l []*weightedWalker
}

func (lh *walkerHeap) Less(i, j int) bool {
	return lh.l[i].W < lh.l[j].W
}

func (lh *walkerHeap) Len() int {
	return len(lh.l)
}

func (lh *walkerHeap) Pop() (loc interface{}) {
	loc, lh.l = lh.l[len(lh.l)-1], lh.l[:len(lh.l)-1]
	return
}

func (lh *walkerHeap) Push(loc interface{}) {
	lh.l = append(lh.l, loc.(*weightedWalker))
}

func (lh *walkerHeap) Swap(i, j int) {
	lh.l[i], lh.l[j] = lh.l[j], lh.l[i]
}

var nInits, nRelease, nAlloc int

type RouteSegment struct {
	Length int
	D      game.Direction
}

type routeSegTreeNode struct {
	RouteSegment
	P int // index of parent node in pool
}

type Route []RouteSegment

func (r Route) Direction(i int) (d game.Direction) {
	for _, v := range r {
		d = v.D
		if v.Length > i {
			return
		}
		i -= v.Length
	}
	panic("index out of range")
}

func (r Route) Len() (len int) {
	for _, v := range r {
		len += v.Length
	}
	return
}

// A* with Jump Points (http://grastien.net/ban/articles/hg-aaai11.pdf)
func NewRoute(w *world.World, start, finish game.Location) (route Route) {
	defer runstat.Record(time.Now(), "NewRoute")
	if start == finish {
		return
	}
	if w.Walls.Get(finish) != 0 {
		// unreachable
		return
	}
	if startRid := w.RoomIds.Get(start); startRid == 0 || startRid != w.RoomIds.Get(finish) {
		// path finding only inside 1 room (1 connected component) for now
		return
	}
	segNodePool := make([]routeSegTreeNode, 0, 100)
	allocateSegNode := func() (n *routeSegTreeNode, idx int) {
		segNodePool = append(segNodePool, routeSegTreeNode{})
		idx = len(segNodePool) - 1
		n = &segNodePool[idx]
		return
	}
	openSetHeap := new(walkerHeap)
	closedSet := layer.NewLayer()
	defer closedSet.Discard()
	gScore := layer.NewLayer()
	defer gScore.Discard()
	openSet := layer.NewLayer()
	defer openSet.Discard()
	initWW := func(ww *weightedWalker, l game.Location) {
		nInits++
		*ww = weightedWalker{
			sc:      layer.NewStackCursor(l),
			SegNode: -1,
		}
		if ww.sc.Add(w.Walls) != wallIndex ||
			ww.sc.Add(gScore) != gScoreIndex ||
			ww.sc.Add(w.ForcedFlags) != flagIndex ||
			ww.sc.Add(openSet) != openIndex ||
			ww.sc.Add(closedSet) != closedIndex {
			panic("unexpected layer index")
		}
	}
	ww := allocate()
	initWW(ww, start)
	heap.Push(openSetHeap, ww)
	firstTile := true
	for openSetHeap.Len() > 0 {
		current := heap.Pop(openSetHeap).(*weightedWalker)
		current.sc.Set(openIndex, 0)
		if current.sc.Cursor() == finish {
			numSegments := 1
			n := &segNodePool[current.SegNode]
			// count number of segments in route
			for n.P != -1 {
				numSegments++
				n = &segNodePool[n.P]
			}
			// reset to end of route and copy segments out
			n = &segNodePool[current.SegNode]
			route = make([]RouteSegment, numSegments)
			for {
				numSegments--
				route[numSegments] = n.RouteSegment
				if n.P == -1 {
					break
				}
				n = &segNodePool[n.P]
			}
			release(&current)
			releaseAll(&openSetHeap.l)
			return
		}
		current.sc.Set(closedIndex, 1)
		closedLocal := current.sc.Look(closedIndex)
		wallLocal := current.sc.Look(wallIndex)
		gLocal := current.sc.Look(gScoreIndex)
		//openLocal := current.OpenReader.Look()
		currentgScore := current.sc.Get(gScoreIndex)
		for d, _ := range current.sc.Cursor().Neighborhood() {
			d := game.Direction(d)
			if closedLocal[d] != 0 || wallLocal[d] != 0 {
				// Neighbor in closed set or blocked by wall
				continue
			}
			if !firstTile && func() bool {
				// Should this direction be pruned?
				// return true -- yes
				// return false -- no
				if d == current.D {
					return false
				}
				switch current.D {
				case game.RIGHT:
					if (d == game.RIGHTUP && wallLocal[game.UP] != 0) ||
						(d == game.RIGHTDOWN && wallLocal[game.DOWN] != 0) {
						return false
					}
				case game.LEFT:
					if (d == game.LEFTUP && wallLocal[game.UP] != 0) ||
						(d == game.LEFTDOWN && wallLocal[game.DOWN] != 0) {
						return false
					}
				case game.UP:
					if (d == game.LEFTUP && wallLocal[game.LEFT] != 0) ||
						(d == game.RIGHTUP && wallLocal[game.RIGHT] != 0) {
						return false
					}
				case game.DOWN:
					if (d == game.LEFTDOWN && wallLocal[game.LEFT] != 0) ||
						(d == game.RIGHTDOWN && wallLocal[game.RIGHT] != 0) {
						return false
					}
				case game.RIGHTUP:
					if d == game.UP || d == game.RIGHT {
						return false
					}
					if (d == game.LEFTUP && wallLocal[game.LEFT] != 0) ||
						(d == game.RIGHTDOWN && wallLocal[game.DOWN] != 0) {
						return false
					}
				case game.LEFTUP:
					if d == game.UP || d == game.LEFT {
						return false
					}
					if (d == game.RIGHTUP && wallLocal[game.RIGHT] != 0) ||
						(d == game.LEFTDOWN && wallLocal[game.DOWN] != 0) {
						return false
					}
				case game.RIGHTDOWN:
					if d == game.DOWN || d == game.RIGHT {
						return false
					}
					if (d == game.LEFTDOWN && wallLocal[game.LEFT] != 0) ||
						(d == game.RIGHTUP && wallLocal[game.UP] != 0) {
						return false
					}
				case game.LEFTDOWN:
					if d == game.DOWN || d == game.LEFT {
						return false
					}
					if (d == game.RIGHTDOWN && wallLocal[game.RIGHT] != 0) ||
						(d == game.LEFTUP && wallLocal[game.UP] != 0) {
						return false
					}
				default:
					panic("invalid direction")
				}
				return true
			}() {
				continue
			}
			var nWalkers *weightedWalker
			nWalkers = allocate()
			nWalkers.Warp(current) // Move nWalkers cursor to current cursor
			nWalkers.D = d
			reachedJump, jumpDist := nWalkers.jump(finish)
			if !reachedJump {
				release(&nWalkers)
				continue
			}
			segNode, idx := allocateSegNode()
			nWalkers.SegNode = idx
			*segNode = routeSegTreeNode{
				RouteSegment: RouteSegment{
					Length: jumpDist,
					D:      d,
				},
				P: current.SegNode,
			}
			if nWalkers.sc.Get(openIndex) == 0 {
				// First visit to this tile, copy current walkers to it, and store score
				nWalkers.sc.Set(gScoreIndex, currentgScore+game.TileId(jumpDist))
				// estimate distance from start to finish through nWalkers.L
				nWalkers.W = int(currentgScore) + jumpDist + nWalkers.sc.Cursor().MaxDistance(finish)
				nWalkers.sc.Set(openIndex, 1)
				heap.Push(openSetHeap, nWalkers)
			} else if int(gLocal[d]) > int(currentgScore)+jumpDist {
				l := nWalkers.sc.Cursor()
				release(&nWalkers)
				// Already visited this tile, but this is a better path to it. Find it
				// in the heap and update its weight.
				index := -1
				for i, p := range openSetHeap.l {
					// TODO this sucks, make it faster
					if p.sc.Cursor() == l {
						// Notify heap of updated weight
						index = i
						nWalkers = p
						break
					}
				}
				if index == -1 {
					panic("can't find tile in priority queue")
				}
				// nWalkers now contains the StackWalker from the previous visit to this neighbor of current
				nWalkers.sc.Set(gScoreIndex, currentgScore+game.TileId(jumpDist))
				// estimate distance from start to finish through nWalkers.L
				nWalkers.W = int(currentgScore) + jumpDist + nWalkers.sc.Cursor().MaxDistance(finish)
				segNodePool[nWalkers.SegNode] = routeSegTreeNode{
					RouteSegment: RouteSegment{
						Length: jumpDist,
						D:      d,
					},
					P: current.SegNode,
				}
				heap.Fix(openSetHeap, index)
			}
		}
		release(&current)
		firstTile = false
	}
	return nil
}
