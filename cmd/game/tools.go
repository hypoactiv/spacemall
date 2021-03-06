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
)

var colorBlue = game.Color{
	R: 0,
	G: 0,
	B: 255,
	A: 255,
}

var colorGreen = game.Color{
	R: 0,
	G: 255,
	B: 0,
	A: 255,
}

var colorRed = game.Color{
	R: 255,
	G: 0,
	B: 0,
	A: 255,
}

var colorWhite = game.Color{
	R: 255,
	G: 255,
	B: 255,
	A: 255,
}

type Tool interface {
	Preview(l game.Location) (<-chan game.Location, game.Color)
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
		Name:   "Conway",
		Create: NewConwayTool,
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

func (p PointTool) Preview(l game.Location) (<-chan game.Location, game.Color) {
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

func (b *TwoPointTool) Preview(l game.Location) (<-chan game.Location, game.Color) {
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

func (t PlaceDoorTool) Preview(l game.Location) (<-chan game.Location, game.Color) {
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

func (t DeleteTool) Preview(l game.Location) (<-chan game.Location, game.Color) {
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

func (t TreeDebugTool) Preview(l game.Location) (<-chan game.Location, game.Color) {
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

func (t *RouteDebugTool) Preview(l game.Location) (c <-chan game.Location, color game.Color) {
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
	a     game.Location
	w     *world.World
	color game.Color
}

func NewRouteWalkerTool(w *world.World) Tool {
	return &RouteWalkerTool{
		w: w,
	}
}

func (t *RouteWalkerTool) Preview(l game.Location) (c <-chan game.Location, color game.Color) {
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
	t.a = l
	t.color = game.RandomColor()
	return nil
}

func (t *RouteWalkerTool) RightClick(l game.Location) game.ModMap {
	rid := t.w.RoomIds.Get(l)
	if rid == 0 {
		return nil
	}
	sc := layer.NewStackCursor(l)
	for i := 0; i < 1000; i++ {
		l := l.JustOffset(rand.Intn(100)-50, rand.Intn(100)-50)
		sc.MoveTo(l)
		if myrid := t.w.RoomIds.Get(l); myrid != rid {
			// only spawn in room 'rid'
			continue
		}
		t.w.Spawn(entity.NewRouteWalker(l, t.a, t.color))
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// Conway Game of Life Cell Tool
type ConwayTool struct {
	a game.Location
	w *world.World
}

func NewConwayTool(w *world.World) Tool {
	return &ConwayTool{
		w: w,
	}
}

func (t *ConwayTool) Preview(l game.Location) (c <-chan game.Location, color game.Color) {
	cc := make(chan game.Location)
	close(cc)
	c = cc
	return
}

func (t *ConwayTool) Click(l game.Location) game.ModMap {
	for i := 0; i < 800; i++ {
		l := l.JustOffset(rand.Intn(50)-10, rand.Intn(50)-10)
		t.w.Spawn(entity.NewConwayCell(l))
	}
	return nil
}

func (t *ConwayTool) RightClick(l game.Location) game.ModMap {
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
