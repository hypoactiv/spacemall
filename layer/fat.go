package layer

import (
	"fmt"
	"jds/game"
	"sync"
)

// Main access modes
// Get -- return specified tiles
// Look -- return 8 neighboring tiles
// Scan -- return the distance to a non-zero tile in a direction
// Collect -- return multiple distances to non-zero tiles in a direction

type layerBlock struct {
	N     [4]*layerBlock // Neighboring blocks, nil if they don't exist yet
	tiles [game.BLOCK_SIZE][game.BLOCK_SIZE]game.TileId
}

func (f *layerBlock) Get(l game.Location) game.TileId {
	return f.tiles[l.Y][l.X]
}

func (f *layerBlock) Set(l game.Location, v game.TileId) {
	f.tiles[l.Y][l.X] = v
}

// returns the pointer to the block (dx,dy), -1<=dx,dy<=1 away from f,
// or nil if it doesn't exist yet, or is unreachable by a rather dumb
// walk.
func (f *layerBlock) Step(x, y int) *layerBlock {
	if x == 0 && y == 0 {
		return f
	}
	progress := true
	for progress { // keep trying until stuck
		progress = false
		for x > 0 && f.N[game.RIGHT] != nil {
			progress = true
			x--
			f = f.N[game.RIGHT]
		}
		for x < 0 && f.N[game.LEFT] != nil {
			progress = true
			x++
			f = f.N[game.LEFT]
		}
		for y > 0 && f.N[game.DOWN] != nil {
			progress = true
			y--
			f = f.N[game.DOWN]
		}
		for y < 0 && f.N[game.UP] != nil {
			progress = true
			y++
			f = f.N[game.UP]
		}
	}
	if x != 0 || y != 0 {
		// failed to reach goal, give up
		f = nil
	}
	return f
}

var (
	blockPool []*layerBlock
)

func releaseBlock(b *layerBlock) {
	blockPool = append(blockPool, b)
}

func allocateBlock() (b *layerBlock) {
	last := len(blockPool) - 1
	if last >= 0 {
		b, blockPool = blockPool[last], blockPool[:last]
		*b = layerBlock{} // Zero memory
	} else {
		b = new(layerBlock)
	}
	return
}

type Blockstore map[game.BlockId]*layerBlock

type Layer struct {
	bs Blockstore
	m  sync.Mutex
}

func (l *Layer) FsckNeighborPointers() {
	for bid, b := range l.bs {
		for d, nbid := range bid.Neighbors() {
			if b.N[d] != l.bs[nbid] {
				panic("neighbor pointers inconsistent")
			}
		}
	}
}

func NewLayer() *Layer {
	return &Layer{
		bs: make(Blockstore),
	}
}

func NewLayerFromSlice(locations []game.Location, v game.TileId) (l *Layer) {
	l = NewLayer()
	l.SetSlice(locations, v)
	return
}

func (l *Layer) SetSlice(locations []game.Location, v game.TileId) {
	for _, ll := range locations {
		l.Set(ll, v)
	}
	return
}

func (l *Layer) InBlockstore(bid game.BlockId) bool {
	_, ok := l.bs[bid]
	return ok
}

// Returns a fatBlock for bid, allocating and initializing if needed
func (l *Layer) fetch(bid game.BlockId) (b *layerBlock) {
	l.m.Lock()
	defer l.m.Unlock()
	b = l.bs[bid]
	if b == nil {
		b = allocateBlock()
		// Link neighbors, if they exist
		for d, nbid := range bid.Neighbors() {
			d := game.Direction(d)
			if nb, ok := l.bs[nbid]; ok {
				// pointer from us to neighbor
				b.N[d] = nb
				// symmetric pointer from neighbor to us
				nb.N[d.Reverse()] = b
			}
		}
		l.bs[bid] = b
	}
	return
}

func (l *Layer) Set(loc game.Location, d game.TileId) {
	if d == 0 && !l.Contains(loc.BlockId) {
		// Trying to set 0, and 0 is the default value for a new block, so don't do anything
		return
	}
	b := l.fetch(loc.BlockId)
	b.Set(loc, d)
}

func (l *Layer) Get(loc game.Location) game.TileId {
	if b := l.bs[loc.BlockId]; b != nil {
		return b.Get(loc)
	}
	return 0
}

func (l *Layer) Contains(bid game.BlockId) bool {
	_, ok := l.bs[bid]
	return ok
}

func (l *Layer) DeepSearch(v game.TileId) (found []game.Location) {
	for bid, lb := range l.bs {
		for i := int8(0); i < game.BLOCK_SIZE; i++ {
			for j := int8(0); j < game.BLOCK_SIZE; j++ {
				if lb.tiles[j][i] == v {
					l := game.Location{
						BlockId: bid,
						X:       i,
						Y:       j,
					}
					found = append(found, l)
				}
			}
		}
	}
	return
}

func (l Layer) DeepSearchNonZero() (found []game.Location) {
	for bid, lb := range l.bs {
		for i := int8(0); i < game.BLOCK_SIZE; i++ {
			for j := int8(0); j < game.BLOCK_SIZE; j++ {
				if lb.tiles[j][i] != 0 {
					l := game.Location{
						BlockId: bid,
						X:       i,
						Y:       j,
					}
					found = append(found, l)
				}
			}
		}
	}
	return
}

// Returns slices of the non-zero values and their distances along a row-mask
func (l *Layer) CollectRowMask(rm *game.RowMask) (values []game.TileId, distances []int) {
	cursor := rm.Left
	b := l.fetch(cursor.BlockId)
	for i := 0; i < rm.Width(); {
		paint, skip := rm.Mask(i)
		if paint {
			if v := b.Get(cursor); v != 0 {
				values = append(values, v)
				distances = append(distances, i)
			}
			cursor.X = (cursor.X + 1) % game.BLOCK_SIZE
			if cursor.X == 0 {
				cursor.BlockId.X++
				b = b.N[game.RIGHT]
				if b == nil {
					b = l.fetch(cursor.BlockId)
				}
			}
			i++
		} else {
			var dx int
			i += skip
			cursor, dx, _ = cursor.Offset(skip, 0)
			b = b.Step(dx, 0)
			if b == nil {
				b = l.fetch(cursor.BlockId)
			}
		}
	}
	return
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

func (l *Layer) SetRowMask(rm *game.RowMask, v game.TileId, m game.ModMap) {
	width := rm.Width()
	b := l.fetch(rm.Left.BlockId)
	cursor := rm.Left
	for i := 0; i < width; {
		p, skip := rm.Mask(i)
		if p {
			for j := 0; j < skip; j++ {
				b.Set(cursor, v)
				cursor.X = (cursor.X + 1) % game.BLOCK_SIZE
				i++
				if cursor.X == 0 {
					cursor.BlockId.X++
					m.AddBlock(cursor.BlockId)
					b = b.N[game.RIGHT]
					if b == nil {
						b = l.fetch(cursor.BlockId)
					}
				}
			}
		} else {
			var dx int
			i += skip
			cursor, dx, _ = cursor.Offset(skip, 0)
			b = b.Step(dx, 0)
			if b == nil {
				b = l.fetch(cursor.BlockId)
			}
		}
	}
}

// Clears a layer. l can be reused.
func (l *Layer) Discard() {
	for _, b := range l.bs {
		releaseBlock(b)
	}
	l.bs = make(Blockstore, len(l.bs)) // if l is recycled, probably going to be about the same size
}

// Allows a stack of layers to be accessed locally with reasonable performance
type StackCursor struct {
	c game.Location
	// len(s) == len(b)
	s []*Layer      // the layers in the stack
	b []*layerBlock // for each layer in the stack, the block which contains c, or nil if it hasn't been loaded yet
}

type LayerIndex int

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

func (sc *StackCursor) FarStep(d game.Direction, distance int) {
	var dx, dy int
	sc.c, dx, dy = sc.c.FarStep(d, distance)
	if dx != 0 || dy != 0 {
		sc.moveBlockPointers(dx, dy)
	}
}

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
	c, dx, dy := sc.c.Step(game.LEFTUP)
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

func (sc *StackCursor) Set(l LayerIndex, v game.TileId) {
	//b := sc.b[l]
	/*	if b == nil {
		b = sc.s[l].fetch(sc.c.BlockId)
		sc.b[l] = b
	}*/
	sc.b[l].Set(sc.c, v)
}

func (sc *StackCursor) Get(l LayerIndex) (v game.TileId) {
	b := sc.b[l]
	/*if b == nil {
		b = sc.s[l].fetch(sc.c.BlockId)
		sc.b[l] = b
	}*/
	v = b.Get(sc.c)
	return
}

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

// Scans layer l in direction d for a non-zero tile
func (sc *StackCursor) Scan(l LayerIndex, d game.Direction, maxDist int) (scanDist int) {
	return sc.scan(l, d, maxDist, 0x7fffffff)
}

// Scans layer l in direction d for a tile with bit bitNum set, i.e. v&(1<<bitNum)!=0
// Returns distance to the found tile, or maxDist if none found.
// If maxDist is -1, there is no limit to scan distance. Function panics if the
// scan ray leaves the world.
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
