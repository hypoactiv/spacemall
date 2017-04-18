package main

import (
	"fmt"
	"jds/game"
	"jds/game/entity"
	"jds/game/layer"
	"jds/game/patterns"
	"jds/game/world"
	"jds/game/world/path"
	"math/rand"
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

var colorBlue = sdl.Color{
	R: 0,
	G: 0,
	B: 255,
	A: 128,
}

var colorGreen = sdl.Color{
	R: 0,
	G: 255,
	B: 0,
	A: 128,
}

var colorRed = sdl.Color{
	R: 255,
	G: 0,
	B: 0,
	A: 128,
}

type Tool interface {
	Preview(l game.Location) (<-chan game.Location, sdl.Color)
	Click(l game.Location) game.ModMap
	RightClick(l game.Location) game.ModMap
}

type ToolCreator func(w *world.World) Tool

var toolset = []struct {
	Name   string
	Create ToolCreator
}{
	{
		Name:   "RouteWalker",
		Create: NewRouteWalkerTool,
	},
	{
		Name:   "Point",
		Create: NewPointTool,
	},
	{
		Name:   "Line",
		Create: NewLineTool,
	},
	{
		Name:   "Box",
		Create: NewBoxTool,
	},
	{
		Name:   "PlaceDoor",
		Create: NewPlaceDoorTool,
	},
	{
		Name:   "Delete",
		Create: NewDeleteTool,
	},
	{
		Name:   "TreeDebug",
		Create: NewTreeDebugTool,
	},
	{
		Name:   "RouteDebug",
		Create: NewRouteDebugTool,
	},
}

///////////////////////////////////////////////////////////////////////////////
// Point Tool
type PointTool struct {
	w *world.World
}

func NewPointTool(w *world.World) Tool {
	return &PointTool{
		w: w,
	}
}

func (p PointTool) Preview(l game.Location) (<-chan game.Location, sdl.Color) {
	c := make(chan game.Location, 1)
	c <- l
	close(c)
	return c, colorGreen
}

func (p PointTool) Click(l game.Location) (m game.ModMap) {
	m = p.w.SetWall(l)
	//m.AddLocation(l)
	return
}

func (p PointTool) RightClick(l game.Location) (m game.ModMap) {
	panic("not implemented")
}

///////////////////////////////////////////////////////////////////////////////
// Tools that take 2 points (Box and line)
type TwoPointDrawer func(a, b game.Location) <-chan game.Location

type TwoPointTool struct {
	// Taking first point, or drawing
	step int
	w    *world.World
	// First point
	a game.Location
	// Drawer function
	drawer TwoPointDrawer
}

func NewLineTool(w *world.World) Tool {
	t := &TwoPointTool{
		w:    w,
		step: 0,
	}
	t.drawer = game.Line
	return t
}

func NewBoxTool(w *world.World) Tool {
	return &TwoPointTool{
		w:      w,
		step:   0,
		drawer: game.Box,
	}
}

func (b *TwoPointTool) Preview(l game.Location) (<-chan game.Location, sdl.Color) {
	if b.step == 0 {
		c := make(chan game.Location, 1)
		c <- l
		close(c)
		return c, colorGreen
	} else if b.step == 1 {
		c := b.drawer(b.a, l)
		return c, colorGreen
	} else {
		panic("wat")
	}
}

func (b *TwoPointTool) Click(l game.Location) (m game.ModMap) {
	if b.step == 0 {
		// Set top left
		b.a = l
		b.step = 1
		return nil
	} else if b.step == 1 {
		m := previewToWalls(b, b.w, l)
		b.step = 0
		return m
	}
	panic("not possible")
}

func (b *TwoPointTool) RightClick(l game.Location) (m game.ModMap) {
	panic("not implemented")
}

///////////////////////////////////////////////////////////////////////////////
// Place Door Tool
type PlaceDoorTool struct {
	w *world.World
}

func NewPlaceDoorTool(w *world.World) Tool {
	return &PlaceDoorTool{
		w: w,
	}
}

func (t PlaceDoorTool) Preview(l game.Location) (<-chan game.Location, sdl.Color) {
	testDoor := func(l game.Location) bool {
		return t.w.CanPlaceDoor(l, world.VERT) || t.w.CanPlaceDoor(l, world.HORZ)
	}
	found, foundLoc := t.w.Walls.FuzzyMatch(l, testDoor)
	w, h := patterns.Door.W, patterns.Door.H()
	color := colorRed
	if found {
		color = colorGreen
		if !t.w.CanPlaceDoor(foundLoc, world.VERT) {
			w, h = h, w
		}
	}
	return game.Box(foundLoc, foundLoc.JustOffset(w-1, h-1)), color
}

func (t PlaceDoorTool) Click(l game.Location) (m game.ModMap) {
	testDoor := func(l game.Location) bool {
		return t.w.CanPlaceDoor(l, world.VERT) || t.w.CanPlaceDoor(l, world.HORZ)
	}
	found, foundLoc := t.w.Walls.FuzzyMatch(l, testDoor)
	if found {
		m = game.NewModMap()
		o := world.Orientation(world.VERT)
		if !t.w.CanPlaceDoor(foundLoc, world.VERT) {
			o = world.HORZ
		}
		t.w.NewDoor(foundLoc, o, m)
	}
	return
}

func (t PlaceDoorTool) RightClick(l game.Location) (m game.ModMap) {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// Delete Tool
type DeleteTool struct {
	w *world.World
}

func NewDeleteTool(w *world.World) Tool {
	return &DeleteTool{
		w: w,
	}
}

func (t DeleteTool) Preview(l game.Location) (<-chan game.Location, sdl.Color) {
	c := make(chan game.Location, 1)
	c <- l
	close(c)
	return c, colorGreen
}

func (t DeleteTool) Click(l game.Location) (m game.ModMap) {
	m = t.w.DeleteFromWallTree(l)
	return
}

func (t DeleteTool) RightClick(l game.Location) (m game.ModMap) {
	panic("not implemented")
}

///////////////////////////////////////////////////////////////////////////////
// Tree Debug Tool
type TreeDebugTool struct {
	w *world.World
}

func NewTreeDebugTool(w *world.World) Tool {
	return &TreeDebugTool{
		w: w,
	}
}

func (t TreeDebugTool) Preview(l game.Location) (<-chan game.Location, sdl.Color) {
	c := make(chan game.Location)
	go func() {
		defer close(c)
		n := t.w.WallNodes[l]
		if n == nil {
			rid := world.RoomId(t.w.RoomIds.Get(l))
			r := t.w.Rooms[rid]
			if r != nil {
				fmt.Println("room-id", rid, "Area", r.Area, "approxArea", r.ApproxArea)
				c <- r.LNPL()
				c <- r.LinkingNode().L
			}
		} else {
			fmt.Println("roomids for", n.L, n.RoomIds)
			for rid, d := range n.RoomIds {
				room := t.w.Rooms[rid]
				fmt.Println("linkage rid", rid, "direction", d, "area", room.Area)
				room.Interior(func(rm *game.RowMask, unused []game.TileId) bool {
					for i := 0; i < rm.Width(); i++ {
						paint, _ := rm.Mask(i)
						if paint {
							c <- rm.Left
						}
						rm.Left, _, _ = rm.Left.Right()
					}
					return true
				})
			}
			for n != nil {
				c <- n.L
				n = n.P
			}
		}
	}()
	return c, colorBlue
}

func (t TreeDebugTool) Click(l game.Location) (m game.ModMap) {
	// Do nothing
	return nil
}

func (t TreeDebugTool) RightClick(l game.Location) (m game.ModMap) {
	// Do nothing
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// Route Debug Tool
type RouteDebugTool struct {
	a game.Location
	w *world.World
}

func NewRouteDebugTool(w *world.World) Tool {
	return &RouteDebugTool{
		w: w,
	}
}

func (t *RouteDebugTool) Preview(l game.Location) (c <-chan game.Location, color sdl.Color) {
	cc := make(chan game.Location)
	c = cc
	r := path.NewRoute(t.w, t.a, l)
	color = colorGreen
	go func() {
		defer close(cc)
		cursor := t.a
		for _, rs := range r {
			for j := uint(0); j < rs.Length; j++ {
				cursor, _, _ = cursor.Step(rs.D)
				cc <- cursor
			}
		}
	}()
	return
}

func (t *RouteDebugTool) Click(l game.Location) game.ModMap {
	fmt.Println(t.a)
	t.a = l
	fmt.Println(t.a)
	return nil
}

func (t *RouteDebugTool) RightClick(l game.Location) game.ModMap {
	panic("not implemented")
}

///////////////////////////////////////////////////////////////////////////////
// Route Walker Tool
type RouteWalkerTool struct {
	a game.Location
	w *world.World
}

func NewRouteWalkerTool(w *world.World) Tool {
	return &RouteWalkerTool{
		w: w,
	}
}

func (t *RouteWalkerTool) Preview(l game.Location) (c <-chan game.Location, color sdl.Color) {
	cc := make(chan game.Location)
	c = cc
	r := path.NewRoute(t.w, t.a, l)
	color = colorGreen
	go func() {
		defer close(cc)
		cursor := t.a
		for _, rs := range r {
			for j := uint(0); j < rs.Length; j++ {
				cursor, _, _ = cursor.Step(rs.D)
				cc <- cursor
			}
		}
	}()
	return
}

func (t *RouteWalkerTool) Click(l game.Location) game.ModMap {
	fmt.Println(t.a)
	t.a = l
	fmt.Println(t.a)
	return nil
}

func (t *RouteWalkerTool) RightClick(l game.Location) game.ModMap {
	rid := t.w.RoomIds.Get(l)
	if rid == 0 {
		return nil
	}
	sc := layer.NewStackCursor(l)
	intentionIndex := sc.Add(t.w.CustomLayer("RouteWalkerIntentions"))
spawnNext:
	for i := 0; i < 10; i++ {
		l := l.JustOffset(rand.Intn(20)-10, rand.Intn(20)-10)
		sc.MoveTo(l)
		//a := t.a.JustOffset(rand.Intn(20)-10, rand.Intn(20)-10)
		if myrid := t.w.RoomIds.Get(l); myrid != rid {
			continue
		}
		if sc.Get(intentionIndex) != 0 {
			continue
		}
		for _, v := range sc.Look(intentionIndex) {
			if v != 0 {
				// don't spawn here, collision possible in the future
				continue spawnNext
			}
		}
		fmt.Println("start spawn", t.w.Now())
		t.w.Spawn(entity.NewRouteWalker(l, t.a))
		thinkStart := time.Now()
		t.w.Think()
		fmt.Println("think took", time.Since(thinkStart))
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////
// Utility functions

// Takes a tool's preview and makes walls out of it
func previewToWalls(t Tool, w *world.World, l game.Location) (m game.ModMap) {
	// Draw box
	c, _ := t.Preview(l)
	for l = range c {
		if m == nil {
			m = w.SetWall(l)
		} else {
			m.Merge(w.SetWall(l))
		}
	}
	return
}
