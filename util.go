package game

// ModMap tracks a list of modified BlockId's. It is used to keep track of
// which blocks to redraw
type ModMap map[BlockId]struct{}

func NewModMap() ModMap {
	return ModMap(make(map[BlockId]struct{}))
}

func (m ModMap) AddBlock(b BlockId) {
	if m == nil {
		return
	}
	m[b] = struct{}{}
}

func (m ModMap) AddRowMask(rm *RowMask) {
	cursor := rm.Left
	for i := 0; i < rm.Width(); {
		paint, skip := rm.Mask(i)
		if paint {
			m.AddBlock(cursor.BlockId)
		}
		for skip >= BLOCK_SIZE {
			skip -= BLOCK_SIZE
			i += BLOCK_SIZE
			cursor.BlockId.X++
			if paint {
				m.AddBlock(cursor.BlockId)
			}
		}
		i += skip
		cursor = cursor.JustOffset(skip, 0)
	}
}

func (m ModMap) AddLocation(l Location) {
	if m == nil {
		return
	}
	m[l.BlockId] = struct{}{}
}

func (m ModMap) Blocks() <-chan BlockId {
	bid := make(chan BlockId)
	if m == nil {
		close(bid)
	} else {
		go func() {
			defer close(bid)
			for k := range m {
				bid <- k
			}
		}()
	}
	return bid
}

func (m ModMap) Merge(n ModMap) {
	for k := range n {
		m[k] = struct{}{}
	}
}
