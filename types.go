package game

import (
	"fmt"
	"math/rand"
)

type Tick int

const (
	// The game world is divided into square blocks with edge length BLOCK_SIZE
	BLOCK_SIZE = 32
)

// The data type of a single value stored in a Layer
type TileId int32

// A Direction is encodes one of the 8 directions, or no movement
type Direction int8

// Directions
const (
	RIGHT = iota
	UP
	DOWN
	LEFT
	RIGHTUP
	RIGHTDOWN
	LEFTUP
	LEFTDOWN

	NONE
)

// Reverses a direction, e.g. Direction(LEFT).Reverse() == RIGHT
func (d Direction) Reverse() Direction {
	return d ^ 0x3
}

func (d Direction) String() string {
	switch d {
	case RIGHT:
		return "Right"
	case UP:
		return "Up"
	case DOWN:
		return "Down"
	case LEFT:
		return "Left"
	case RIGHTUP:
		return "RightUp"
	case RIGHTDOWN:
		return "RightDown"
	case LEFTUP:
		return "LeftUp"
	case LEFTDOWN:
		return "LeftDown"
	case NONE:
		return "None"
	default:
		return "Invalid direction"
	}
}

// A BlockId identifies a block of tiles on the "infinite" 2d plane
type BlockId struct {
	X, Y int
}

func (b BlockId) Neighbors() [4]BlockId {
	return [4]BlockId{
		b.RightBlock(),
		b.UpBlock(),
		b.DownBlock(),
		b.LeftBlock(),
	}
}

func (b BlockId) Step(d Direction) (bb BlockId) {
	bb = b
	switch d {
	case RIGHT:
		bb.X++
	case UP:
		bb.Y--
	case DOWN:
		bb.Y++
	case LEFT:
		bb.X--
	default:
		panic("invalid direction")
	}
	return
}

// Location uniquely identifies a single tile in the world
type Location struct {
	X, Y    int8
	BlockId BlockId
}

// Iterate over all tiles in the block
func (b BlockId) Iterate() <-chan Location {
	c := make(chan Location)
	go func() {
		defer close(c)
		l := Location{
			BlockId: b,
		}
		for x := int8(0); x < BLOCK_SIZE; x++ {
			for y := int8(0); y < BLOCK_SIZE; y++ {
				l.X = x
				l.Y = y
				c <- l
			}
		}
	}()
	return c
}

func (l Location) SmallDistance(ll Location) (int, int) {
	x, y := l.Distance(ll)
	return int(x), int(y)
}

func (l Location) Distance(ll Location) (x, y int64) {
	return int64(ll.BlockId.X-l.BlockId.X)*BLOCK_SIZE + int64(ll.X-l.X), int64(ll.BlockId.Y-l.BlockId.Y)*BLOCK_SIZE + int64(ll.Y-l.Y)
}

func (l Location) FarStep(d Direction, distance int) (Location, int, int) {
	switch d {
	case RIGHT:
		return l.Offset(distance, 0)
	case UP:
		return l.Offset(0, -distance)
	case DOWN:
		return l.Offset(0, distance)
	case LEFT:
		return l.Offset(-distance, 0)
	case RIGHTUP:
		return l.Offset(distance, -distance)
	case LEFTUP:
		return l.Offset(-distance, -distance)
	case RIGHTDOWN:
		return l.Offset(-distance, distance)
	case LEFTDOWN:
		return l.Offset(-distance, distance)
	default:
		panic("invalid direction")
	}
}

// L-Infinity norm
func (l Location) MaxDistance(ll Location) int {
	x, y := l.SmallDistance(ll)
	if x < 0 {
		x = -x
	}
	if y < 0 {
		y = -y
	}
	if x > y {
		return x
	}
	return y
}

// L-2 squared Norm
func (l Location) LinearDistance(ll Location) int {
	x, y := l.SmallDistance(ll)
	return x*x + y*y
}

// L-1 Norm
func (l Location) AbsDistance(ll Location) int {
	x, y := l.SmallDistance(ll)
	if x < 0 {
		x = -x
	}
	if y < 0 {
		y = -y
	}
	return x + y
}

func (l Location) String() string {
	return fmt.Sprintf("%d@%d,%d", l.BlockId, l.X, l.Y)
}

func (l *Location) fixup() (ret bool) {
	if l.X >= BLOCK_SIZE {
		l.X -= BLOCK_SIZE
		l.BlockId.X++
		ret = true
	}
	if l.Y >= BLOCK_SIZE {
		l.Y -= BLOCK_SIZE
		l.BlockId.Y++
		ret = true
	}
	if l.X < 0 {
		l.X += BLOCK_SIZE
		l.BlockId.X--
		ret = true
	}
	if l.Y < 0 {
		l.Y += BLOCK_SIZE
		l.BlockId.Y--
		ret = true
	}
	return
}

func (l Location) JustOffset(relx, rely int) (ll Location) {
	ll, _, _ = l.Offset(relx, rely)
	return
}

// Offset the location by the specified x- and y-distances
// returns the new location, and layerBlock delta-x and -y
func (l Location) Offset(relx, rely int) (Location, int, int) {
	blockx := relx / BLOCK_SIZE
	blocky := rely / BLOCK_SIZE
	ll := Location{
		BlockId: BlockId{
			X: l.BlockId.X + blockx,
			Y: l.BlockId.Y + blocky,
		},
		X: int8(int(l.X) + relx - blockx*BLOCK_SIZE),
		Y: int8(int(l.Y) + rely - blocky*BLOCK_SIZE),
	}
	ll.fixup()
	return ll, ll.BlockId.X - l.BlockId.X, ll.BlockId.Y - l.BlockId.Y
}

func (b BlockId) RightBlock() BlockId {
	return BlockId{
		X: b.X + 1,
		Y: b.Y,
	}
}

func (b BlockId) UpBlock() BlockId {
	return BlockId{
		X: b.X,
		Y: b.Y - 1,
	}
}

func (b BlockId) LeftBlock() BlockId {
	return BlockId{
		X: b.X - 1,
		Y: b.Y,
	}
}

func (b BlockId) DownBlock() BlockId {
	return BlockId{
		X: b.X,
		Y: b.Y + 1,
	}
}

func (l Location) Right() (Location, int, int) {
	l.X = (l.X + 1) % BLOCK_SIZE
	if l.X == 0 {
		l.BlockId.X++
		return l, 1, 0
	}
	return l, 0, 0
}

func (l Location) Up() (Location, int, int) {
	l.Y = (l.Y - 1 + BLOCK_SIZE) % BLOCK_SIZE
	if l.Y == BLOCK_SIZE-1 {
		l.BlockId.Y--
		return l, 0, -1
	}
	return l, 0, 0
}

func (l Location) Left() (Location, int, int) {
	l.X = (l.X - 1 + BLOCK_SIZE) % BLOCK_SIZE
	if l.X == BLOCK_SIZE-1 {
		l.BlockId.X--
		return l, -1, 0
	}
	return l, 0, 0
}

func (l Location) Down() (Location, int, int) {
	l.Y = (l.Y + 1) % BLOCK_SIZE
	if l.Y == 0 {
		l.BlockId.Y++
		return l, 0, 1
	}
	return l, 0, 0
}

func (l Location) RightUp() (Location, int, int) {
	dx, dy := 0, 0
	l.X = (l.X + 1) % BLOCK_SIZE
	if l.X == 0 {
		l.BlockId.X++
		dx = 1
	}
	l.Y = (l.Y - 1 + BLOCK_SIZE) % BLOCK_SIZE
	if l.Y == BLOCK_SIZE-1 {
		l.BlockId.Y--
		dy = -1
	}
	return l, dx, dy
}

func (l Location) LeftUp() (Location, int, int) {
	dx, dy := 0, 0
	l.X = (l.X - 1 + BLOCK_SIZE) % BLOCK_SIZE
	if l.X == BLOCK_SIZE-1 {
		l.BlockId.X--
		dx = -1
	}
	l.Y = (l.Y - 1 + BLOCK_SIZE) % BLOCK_SIZE
	if l.Y == BLOCK_SIZE-1 {
		l.BlockId.Y--
		dy = -1
	}
	return l, dx, dy
}

func (l Location) RightDown() (Location, int, int) {
	dx, dy := 0, 0
	l.X = (l.X + 1) % BLOCK_SIZE
	if l.X == 0 {
		l.BlockId.X++
		dx = 1
	}
	l.Y = (l.Y + 1) % BLOCK_SIZE
	if l.Y == 0 {
		l.BlockId.Y++
		dy = 1
	}
	return l, dx, dy
}

func (l Location) LeftDown() (Location, int, int) {
	dx, dy := 0, 0
	l.X = (l.X - 1 + BLOCK_SIZE) % BLOCK_SIZE
	if l.X == BLOCK_SIZE-1 {
		l.BlockId.X--
		dx = -1
	}
	l.Y = (l.Y + 1) % BLOCK_SIZE
	if l.Y == 0 {
		l.BlockId.Y++
		dy = 1
	}
	return l, dx, dy
}

func (l Location) JustStep(d Direction) (ll Location) {
	ll, _, _ = l.Step(d)
	return
}

// Returns new location, and BlockId delta X and Y.
func (l Location) Step(d Direction) (Location, int, int) {
	switch d {
	case UP:
		return l.Up()
	case DOWN:
		return l.Down()
	case LEFT:
		return l.Left()
	case RIGHT:
		return l.Right()
	case RIGHTUP:
		return l.RightUp()
	case RIGHTDOWN:
		return l.RightDown()
	case LEFTUP:
		return l.LeftUp()
	case LEFTDOWN:
		return l.LeftDown()
	case NONE:
		return l, 0, 0
	default:
		panic("invalid direction")
	}
}

func (l Location) Neighbors() (n [4]Location) {
	n[RIGHT], _, _ = l.Right()
	n[UP], _, _ = l.Up()
	n[DOWN], _, _ = l.Down()
	n[LEFT], _, _ = l.Left()
	return
}

func (l Location) Neighborhood() (n [8]Location) {
	n[RIGHT], _, _ = l.Right()
	n[UP], _, _ = l.Up()
	n[DOWN], _, _ = l.Down()
	n[LEFT], _, _ = l.Left()
	n[RIGHTUP], _, _ = l.RightUp()
	n[LEFTUP], _, _ = l.LeftUp()
	n[RIGHTDOWN], _, _ = l.RightDown()
	n[LEFTDOWN], _, _ = l.LeftDown()
	return
}

func Line(a, b Location) <-chan Location {
	//modified = make(map[BlockId]bool)
	x, y := a.Distance(b)
	if x < 0 {
		a, b = b, a
		x, y = a.Distance(b)
	}
	c := make(chan Location)
	go func() {
		defer close(c)
		j := int64(0)
		for i := int64(0); i <= x; i++ {
			var yt int64
			c <- a
			if x == 0 {
				yt = y
			} else {
				yt = y * i / x
			}
			for j != yt {
				if yt > 0 {
					a, _, _ = a.Down()
					j++
				} else {
					a, _, _ = a.Up()
					j--
				}
				c <- a
			}
			a, _, _ = a.Right()
		}
	}()
	return c
}

func Box(a, b Location) <-chan Location {
	x, y := a.SmallDistance(b)
	a1, _, _ := a.Offset(x, 0)
	a2, _, _ := a.Offset(0, y)
	c1 := Line(a, a1)
	c2 := Line(a1, b)
	c3 := Line(b, a2)
	c4 := Line(a2, a)
	c := make(chan Location)
	go func() {
		defer close(c)
		for _, cc := range [4]<-chan Location{c1, c2, c3, c4} {
			for l := range cc {
				c <- l
			}
		}
	}()
	return c
}

// Returns the direction to move from a towards b along the path of steepest
// descent of MaxDistance
func (a Location) Towards(b Location) Direction {
	dx, dy := a.Distance(b) // b - a
	switch {
	case dx > 0 && dy > 0:
		return RIGHTDOWN
	case dx == 0 && dy > 0:
		return DOWN
	case dx < 0 && dy > 0:
		return LEFTDOWN
	case dx > 0 && dy < 0:
		return RIGHTUP
	case dx == 0 && dy < 0:
		return UP
	case dx < 0 && dy < 0:
		return LEFTUP
	case dx > 0 && dy == 0:
		return RIGHT
	case dx < 0 && dy == 0:
		return LEFT
	default:
		return NONE
	}
}

type Color struct {
	R, G, B, A uint8
}

func RandomColor() Color {
	return Color{
		R: uint8(rand.Intn(255)),
		G: uint8(rand.Intn(255)),
		B: uint8(rand.Intn(255)),
		A: 255,
	}
}
