package layer

import (
	"jds/game"
	"jds/game/patterns"
	"sync"
)

// Main access modes
// Get -- return specified tiles
// Look -- return 8 neighboring tiles
// Scan -- return the distance to a non-zero tile in a direction
// Collect -- return multiple distances to non-zero tiles in a direction

type Blockstore map[game.BlockId]*layerBlock

// A Layer is an infinite grid of TileIds, subdivided into squares (Blocks) of
// edge length game.BLOCK_SIZE. All TileIds are initially 0.
type Layer struct {
	bs Blockstore
	m  sync.Mutex
}

// Creates a new Layer
func NewLayer() *Layer {
	return &Layer{
		bs: make(Blockstore),
	}
}

type layerBlock struct {
	N     [4]*layerBlock // Neighboring blocks, nil if they don't exist yet
	tiles [game.BLOCK_SIZE][game.BLOCK_SIZE]game.TileId
}

// Set the TileId of Location loc
func (l *Layer) Set(loc game.Location, d game.TileId) {
	b := l.fetch(loc.BlockId)
	b.Set(loc, d)
}

// Get the TileId of Location loc
func (l *Layer) Get(loc game.Location) game.TileId {
	if b := l.bs[loc.BlockId]; b != nil {
		return b.Get(loc)
	}
	return 0
}

// Returns a slice of Locations which have the same TileId as loc
func (l *Layer) Flood(loc game.Location) (flood []game.Location) {
	v := l.Get(loc)
	if v == 0 {
		panic("tried to flood 0")
	}
	q := make([]game.Location, 1)
	q[0] = loc
	memory := make(map[game.Location]bool)
	memory[loc] = true
	for len(q) > 0 {
		loc, q = q[0], q[1:]
		for _, n := range loc.Neighborhood() {
			if memory[n] == false && l.Get(n) == v {
				memory[n] = true
				q = append(q, n)
			}
		}
		flood = append(flood, loc)
	}
	return
}

// Returns true if the pattern p appears in layer l with upper-left corner at loc
func (l *Layer) Match(loc game.Location, p patterns.Pattern, transpose bool) bool {
	sc := NewStackCursor(loc)
	li := sc.Add(l)
	for i, pv := range p.P {
		x := i % p.W
		y := i / p.W
		if transpose {
			x, y = y, x
		}
		if sc.OffsetGet(li, x, y) != pv {
			return false
		}
	}
	return true
}

// Returns the Location l closest (using AbsDistance) to loc at which test(l)
// is true. If no match is found in the search radius, return false
func (l *Layer) FuzzyMatch(loc game.Location, test func(game.Location) bool) (found bool, at game.Location) {
	if test(loc) {
		return true, loc
	}
	at = loc
	bestDist := 10
	for i := -2; i <= 2; i++ {
		for j := -2; j <= 2; j++ {
			cursor := loc.JustOffset(i, j)
			if dist := cursor.AbsDistance(loc); dist < bestDist && test(cursor) {
				// new best location found
				at = cursor
				bestDist = dist
				found = true
			}
		}
	}
	return
}

// Searches for pattern p in Layer l within a radius of Location loc
func (l *Layer) FuzzyMatchPattern(loc game.Location, p patterns.Pattern) (found bool, at game.Location, transpose bool) {
	test := func(testLoc game.Location) bool {
		return l.Match(testLoc, p, true) || l.Match(testLoc, p, false)
	}
	found, at = l.FuzzyMatch(loc, test)
	if found {
		if !l.Match(at, p, false) {
			// only choose transpose if non-transpose fails
			transpose = true
		}
		return
	}
	return
}

// Sets value v according to the non-zero locations of p, with upper left corner at loc
func (l *Layer) SetMask(loc game.Location, p patterns.Pattern, transpose bool, v game.TileId, m game.ModMap) {
	for i, pv := range p.P {
		x := i % p.W
		y := i / p.W
		if transpose {
			x, y = y, x
		}
		if pv != 0 {
			ll := loc.JustOffset(x, y)
			l.Set(ll, v)
			m.AddBlock(ll.BlockId)
		}
	}
}

func (f *layerBlock) Get(l game.Location) game.TileId {
	return f.tiles[l.Y][l.X]
}

func (f *layerBlock) Set(l game.Location, v game.TileId) {
	f.tiles[l.Y][l.X] = v
}

// Attempts to return the pointer to the block (x,y) blocks away from 'f'
// If the destination block is unreachable by a direct walk, returns 'nil'
//
// TODO instead of returning nil, call fetch, so that Step always succeeds?
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

// layerBlock Pool
var blockPool sync.Pool

func releaseBlock(b *layerBlock) {
	blockPool.Put(b)
}

func allocateBlock() (b *layerBlock) {
	var ok bool
	if b, ok = blockPool.Get().(*layerBlock); !ok {
		return new(layerBlock)
	}
	// Zero recycled block
	*b = layerBlock{}
	return
}

// Verify integrity of neighbor Block pointers (debug)
func (l *Layer) FsckNeighborPointers() {
	for bid, b := range l.bs {
		for d, nbid := range bid.Neighbors() {
			if b.N[d] != l.bs[nbid] {
				panic("neighbor pointers inconsistent")
			}
		}
	}
}

// Returns a new Layer with the provided locations set to TileId v
func NewLayerFromSlice(locations []game.Location, v game.TileId) (l *Layer) {
	l = NewLayer()
	l.SetSlice(locations, v)
	return
}

// Sets the slice of Locations to TileId v
func (l *Layer) SetSlice(locations []game.Location, v game.TileId) {
	for _, ll := range locations {
		l.Set(ll, v)
	}
	return
}

// Returns true if the Layer possibly has non-zero values in BlockId
func (l *Layer) InBlockstore(bid game.BlockId) bool {
	_, ok := l.bs[bid]
	return ok
}

// Returns a layerBlock for bid, allocating and initializing if needed
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
