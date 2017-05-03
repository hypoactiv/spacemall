package layer

import (
	"fmt"
	"jds/game"
)

// The index of a single layer accessible by a StackCursor
type LayerIndex int

// Allows a set of layers (a 'stack' of layers) to be accessed near a common
// game location (the 'cursor') with reasonable performance. Maintains a cache
// of pointers to the layerBlocks containing the cursor location.
type StackCursor struct {
	c      game.Location   //cursor
	s      []*Layer        // the layers in the stack
	b      []*layerBlock   // for each layer in the stack, the block which contains c	// len(s) == len(b)
	cStack []game.Location // saved cursor position stack
}

// Fills 'row' with len(row) values to the right of sc.Cursor()
func (sc *StackCursor) GetRow(l LayerIndex, row []game.TileId) {
	for i := range row {
		row[i] = 0
	}
	width := len(row)
	left := sc.c
	bid := left.BlockId
	x, y := int(left.X), int(left.Y)
	sl := sc.s[l]
	b := sc.b[l]
	/*if b == nil {
		b = sl.bs[bid]
		sc.b[l] = b
	}*/
	i := 0
	// Copy first partial block row, if any
	if x > 0 {
		if b == nil {
			x = 0
			i += game.BLOCK_SIZE - x
			bid.X++
			b = sl.bs[bid]
		} else {
			for x > 0 && i < width {
				row[i] = b.tiles[y][x]
				x = (x + 1) % game.BLOCK_SIZE
				i++
			}
			bid.X++
			b = b.N[game.RIGHT]
		}
	}
	// TODO remove
	if x != 0 && i < width {
		fmt.Println(x)
		panic("x nonzero")
	}
	for i < width {
		if i < width-game.BLOCK_SIZE {
			// at least a block remaining
			if b == nil {
				// nothing here, skip to next block
				i += game.BLOCK_SIZE
				bid.X++
				b = sl.bs[bid] // don't use fetch because it allocates
			} else {
				// copy block row
				// i < width-BLOCK_SIZE < width
				for _, v := range b.tiles[y] {
					row[i] = v
					i++ // increments i at most BLOCK_SIZE times, so i < width
				}
				bid.X++
				b = b.N[game.RIGHT]
			}
		} else {
			// less than a block remaining
			if b == nil {
				break
			} else {
				for _, v := range b.tiles[y] {
					row[i] = v
					i++ // increments i BLOCK_SIZE times
					if i >= width {
						break
					}
				}
			}
		}
	}
}

func NewStackCursor(start game.Location) StackCursor {
	return StackCursor{
		c: start,
		s: make([]*Layer, 0, 5),
		b: make([]*layerBlock, 0, 5),
	}
}

func MoveStackCursor(from *StackCursor, to *StackCursor) {
	// copy cursor
	from.c = to.c
	// copy block pointers
	from.b = from.b[:0]
	from.b = append(from.b, to.b...)
	// copy Layer pointers
	from.s = from.s[:0]
	from.s = append(from.s, to.s...)
}

// Returns true if layer l has a non-zero value on the steepest-descent path
// from the StackCursor's current position to a.
func (sc *StackCursor) Obstructed(l LayerIndex, a game.Location) bool {
	sc.Push()
	for sc.c.MaxDistance(a) > 0 {
		if sc.Get(l) != 0 {
			sc.Pop()
			return true
		}
		sc.Step(sc.c.Towards(a))
	}
	sc.Pop()
	return false
}

// Push current cursor location onto location stack
func (sc *StackCursor) Push() {
	sc.cStack = append(sc.cStack, sc.c)
}

// Pop location off location stack
func (sc *StackCursor) Pop() {
	var c game.Location
	last := len(sc.cStack) - 1
	c, sc.cStack = sc.cStack[last], sc.cStack[:last]
	sc.MoveTo(c)
}

// Moves the StackCursor 'distance' tiles in direction 'd'
func (sc *StackCursor) FarStep(d game.Direction, distance int) {
	var dx, dy int
	sc.c, dx, dy = sc.c.FarStep(d, distance)
	if dx != 0 || dy != 0 {
		sc.moveBlockPointers(dx, dy)
	}
}

// Moves the StackCursor to the specified Location
func (sc *StackCursor) MoveTo(l game.Location) {
	dx, dy := sc.Cursor().SmallDistance(l)
	sc.c, dx, dy = sc.c.Offset(dx, dy)
	if dx != 0 || dy != 0 {
		sc.moveBlockPointers(dx, dy)
	}
}

func (sc *StackCursor) Dump() {
	fmt.Println("internal cursor", sc.c)
	for l := range sc.b {
		for k, v := range sc.s[l].bs {
			if v == sc.b[l] {
				fmt.Println("layer", l, "block for", k)
			}
		}
	}
}

// Add a layer to the stack. LayerIndex guaranteed to start at 0 and increase
func (sc *StackCursor) Add(l *Layer) LayerIndex {
	sc.b = append(sc.b, l.fetch(sc.c.BlockId))
	sc.s = append(sc.s, l)
	return LayerIndex(len(sc.s) - 1)
}

func (sc *StackCursor) moveBlockPointers(dx, dy int) {
	// update layerBlock pointer for each layer in the stack
	for i := range sc.b {
		i := LayerIndex(i)
		b := sc.b[i]
		b = b.Step(dx, dy)
		if b == nil {
			b = sc.s[i].fetch(sc.c.BlockId)
		}
		sc.b[i] = b
	}
}

// Step (move) the cursor
func (sc *StackCursor) Step(d game.Direction) {
	var dx, dy int
	sc.c, dx, dy = sc.c.Step(d)
	if dx != 0 || dy != 0 {
		sc.moveBlockPointers(dx, dy)
	}
}

// Returns current cursor location
func (sc *StackCursor) Cursor() game.Location {
	return sc.c
}

// Gets from a location dx, dy away from sc's cursor
func (sc *StackCursor) OffsetGet(l LayerIndex, dx, dy int) game.TileId {
	c := sc.c
	farC, blockDx, blockDy := c.Offset(dx, dy)
	b := sc.b[l]
	if b != nil {
		b = b.Step(blockDx, blockDy)
	}
	if b == nil {
		return sc.s[l].Get(farC)
	}
	return b.tiles[farC.Y][farC.X]
}

// Gets from a location distance tiles away in direction d from sc's cursor
func (sc *StackCursor) FarStepGet(l LayerIndex, d game.Direction, distance int) game.TileId {
	if d >= 4 {
		panic("diagonals not implemented")
	}
	c := sc.c
	farC, dx, dy := c.FarStep(d, distance)
	b := sc.b[l]
	if b != nil {
		b = b.Step(dx, dy)
	}
	if b == nil {
		// give up, use a slow Get
		return sc.s[l].Get(farC)
	}
	return b.tiles[farC.Y][farC.X]
}

// Get layer values around cursor from specified layer
func (sc *StackCursor) Look(l LayerIndex) (proximity [8]game.TileId) {
	xOffsets := [8]int8{2, 1, 1, 0, 2, 2, 0, 0}
	yOffsets := [8]int8{1, 0, 2, 1, 0, 2, 0, 2}
	sl := sc.s[l]
	// Step cursor LEFTUP so all offsets above are positive, and only 4 blocks need to be considered
	c, dx, dy := sc.c.LeftUp()
	// try to reach c's block by sc.c's block
	bc := sc.b[l]
	if bc == nil {
		bc = sl.bs[sc.c.BlockId]
		sc.b[l] = bc
	}
	if bc != nil {
		bc = bc.Step(dx, dy)
	}
	if bc == nil {
		// give up, consult the hash table or create the block
		bc = sl.fetch(c.BlockId)
	}
	// bc != nil at this point
	// c is in the top left corner of the 3x3 block to be read, and bc is a pointer to the block containing c
	x := c.X
	y := c.Y
	bid := c.BlockId
	if x < game.BLOCK_SIZE-2 && y < game.BLOCK_SIZE-2 {
		// Easy (and common) case, entire read contained in 1 block
		for i := range proximity {
			proximity[i] = bc.tiles[y+yOffsets[i]][x+xOffsets[i]]
		}
	} else {
		// Read spans blocks
		b := [4]*layerBlock{
			bc,
			bc.Step(1, 0),
			bc.Step(0, 1),
			bc.Step(1, 1),
		}
		// neighboring blocks may not be loaded
		if b[1] == nil {
			b[1] = sl.fetch(bid.RightBlock())
		}
		if b[2] == nil {
			b[2] = sl.fetch(bid.DownBlock())
		}
		if b[3] == nil {
			b[3] = sl.fetch(bid.DownBlock().RightBlock())
		}
		for i := range proximity {
			bIndex := 0
			xx := x + xOffsets[i]
			yy := y + yOffsets[i]
			if xx >= game.BLOCK_SIZE {
				bIndex = 1
			}
			if yy >= game.BLOCK_SIZE {
				bIndex |= 2
			}
			if b[bIndex] == nil {
				continue
			}
			proximity[i] = b[bIndex].tiles[yy%game.BLOCK_SIZE][xx%game.BLOCK_SIZE]
		}
	}
	return
}

// Set value at cursor in specified layer
func (sc *StackCursor) Set(l LayerIndex, v game.TileId) {
	//b := sc.b[l]
	/*	if b == nil {
		b = sc.s[l].fetch(sc.c.BlockId)
		sc.b[l] = b
	}*/
	sc.b[l].Set(sc.c, v)
}

// Get value at cursor in specified layer
func (sc *StackCursor) Get(l LayerIndex) (v game.TileId) {
	b := sc.b[l]
	/*if b == nil {
		b = sc.s[l].fetch(sc.c.BlockId)
		sc.b[l] = b
	}*/
	v = b.Get(sc.c)
	return
}

// Set or clear bit at cursor in specified layer. If v is true, the bit is
// set, otherwise the bit is cleared.
func (sc *StackCursor) SetBit(l LayerIndex, bit uint, v bool) {
	if v {
		// set bit
		sc.b[l].Set(sc.c, sc.b[l].Get(sc.c)|(1<<bit))
	} else {
		// clear bit
		sc.b[l].Set(sc.c, sc.b[l].Get(sc.c)&^(1<<bit))
	}
}

// Get bit at cursor in specified layer
func (sc *StackCursor) GetBit(l LayerIndex, bit uint) (v bool) {
	if sc.b[l].Get(sc.c)&(1<<bit) != 0 {
		v = true
	}
	return
}

// Get bit at cursor's neighbot in specified layer
func (sc *StackCursor) DirectedGetBit(l LayerIndex, d game.Direction, bit uint) (v bool) {
	if sc.DirectedGet(l, d)&(1<<bit) != 0 {
		v = true
	}
	return
}

// Get value from cursor's neighbor in direction 'd'
func (sc *StackCursor) DirectedGet(l LayerIndex, d game.Direction) (v game.TileId) {
	c, dx, dy := sc.c.Step(d)
	b := sc.b[l]
	if dx != 0 || dy != 0 {
		b = b.Step(dx, dy)
		if b == nil {
			b = sc.s[l].fetch(c.BlockId)
		}
	}
	return b.Get(c)
}

// Set value of cursor's neighbor in direction 'd'
func (sc *StackCursor) DirectedSet(l LayerIndex, d game.Direction, v game.TileId) {
	c, dx, dy := sc.c.Step(d)
	b := sc.b[l]
	if dx != 0 || dy != 0 {
		b = b.Step(dx, dy)
		if b == nil {
			b = sc.s[l].fetch(c.BlockId)
		}
	}
	b.Set(c, v)
}

// Scans layer l in direction d for a non-zero tile
func (sc *StackCursor) Scan(l LayerIndex, d game.Direction, maxDist int) (scanDist int) {
	return sc.scan(l, d, maxDist, 0x7fffffff)
}

// Scans layer l in direction d for a tile with bit bitNum set, i.e. v&(1<<bitNum)!=0
// Returns distance to the found tile, or maxDist if none found.
// If maxDist is -1, there is no limit to scan distance. Function panics if the
// scan ray appears to leave the world.
func (sc *StackCursor) ScanBit(l LayerIndex, d game.Direction, maxDist int, bitNum uint) (scanDist int) {
	return sc.scan(l, d, maxDist, 1<<bitNum)
}

func (sc *StackCursor) scan(l LayerIndex, d game.Direction, maxDist int, mask game.TileId) (scanDist int) {
	if d >= 4 {
		panic("diagonal direction scanning not implemented")
	}
	sl := sc.s[l]
	// load cursor block
	b := sc.b[l]
	/*if b == nil {
		b = sl.bs[sc.c.BlockId]
		sc.b[l] = b
	}*/
	c := sc.c
	//fmt.Println("scan start", c, d, mask)
	switch d {
	case game.RIGHT:
		scanDist = -int(c.X)
	case game.UP:
		scanDist = int(c.Y) - game.BLOCK_SIZE + 1
	case game.DOWN:
		scanDist = -int(c.Y)
	case game.LEFT:
		scanDist = int(c.X) - game.BLOCK_SIZE + 1
	default:
		panic("asdF")
	}
	for {
		if maxDist >= 0 && scanDist == maxDist {
			return
		}
		skippedBlocks := 0
		for b == nil {
			//fmt.Println("skip block")
			// Skip empty blocks
			skippedBlocks++
			c.BlockId = c.BlockId.Step(d)
			b = sl.bs[c.BlockId]
			scanDist += game.BLOCK_SIZE
			if maxDist >= 0 && scanDist >= maxDist {
				scanDist = maxDist
				return
			}
			if maxDist == -1 && skippedBlocks > 10 && skippedBlocks > len(sl.bs) {
				// now concerned ray has left the world
				// See if there's any block in direction d
				panic("not implemented -- ray left world?")
				//for k, v := range sl.bs {
				//is k in direction d with positive distance? if so, jump to it and continue scanning.
				//}
				//no more blocks in direction d, ray left world, panic
			}
		}
		switch d {

		case game.RIGHT, game.LEFT:
			// scan block row
			x := 0
			if d == game.LEFT {
				x = game.BLOCK_SIZE - 1
			}
			for i := 0; i < game.BLOCK_SIZE; i++ {
				//fmt.Println("scan row", i, x, c.X, v, scanDist)
				if b.tiles[c.Y][x]&mask != 0 {
					//fmt.Println("hit", i, scanDist)
					// found bit set in this block at Y=c.Y and X=i
					if maxDist >= 0 && scanDist > maxDist {
						scanDist = maxDist
					}
					if scanDist >= 0 { // if scanDist is negative, the tile found is in opposite direction. continue searching
						//fmt.Println("done", scanDist)
						return
					}
				}
				scanDist++
				if d == game.RIGHT {
					// scan left to right
					x++
				} else {
					// d == game.LEFT
					// scan right to left
					x--
				}
				//fmt.Println(x)
			}
		case game.UP, game.DOWN:
			// scan block col
			y := 0
			if d == game.UP {
				y = game.BLOCK_SIZE - 1
			}
			for i := 0; i < game.BLOCK_SIZE; i++ {
				//fmt.Println("scan col", i, y, c.Y, v, scanDist)
				//fmt.Println(y)
				if b.tiles[y][c.X]&mask != 0 {
					//fmt.Println("hit", i, scanDist)
					// found bit set in this block at Y=c.Y and X=i
					if maxDist >= 0 && scanDist > maxDist {
						scanDist = maxDist
					}
					if scanDist >= 0 { // if scanDist is negative, the tile found is in opposite direction. continue searching
						//fmt.Println("done", scanDist)
						return
					}
				}
				scanDist++
				if d == game.DOWN {
					// scan top to bottom
					y++
				} else {
					// d == game.UP
					// scan bottom to top
					y--
				}
			}
		}
		// nothing found in this block, continue in next
		//fmt.Printf("orig %p %v %v\n", b, b.N, d)
		b = b.N[d]
		c.BlockId = c.BlockId.Step(d)
	}
}

// Debug
func (sc *StackCursor) Depth() int {
	return len(sc.s)
}
