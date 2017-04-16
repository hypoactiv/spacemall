package layer

import (
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
