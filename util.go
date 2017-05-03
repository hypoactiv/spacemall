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

// Tracks the minimum among multiple observed values, and the argument
// associated with that value
type Min struct {
	min      int
	argmin   interface{}
	feasible bool
}

// Observe an argument and its value
func (m *Min) Observe(arg interface{}, value int) {
	if value < m.min || !m.feasible {
		// New minimum
		m.feasible = true
		m.min = value
		m.argmin = arg
	}
}

// Returns true if the Min has observed at least one value
func (m *Min) Feasible() bool {
	return m.feasible
}

// Returns true if the minimum will be improved by this value
func (m *Min) ImprovedBy(value int) bool {
	return value < m.min || !m.feasible
}

// Returns the argument associated with the miniumum observed (arg,value) pairs
//
// Returns nil if there have been no observations
func (m *Min) Argmin() interface{} {
	return m.argmin
}

// Returns the minimum value of the observed (arg,value) pairs
//
// Panics if there have not been any observations
func (m *Min) Min() int {
	if !m.feasible {
		panic("no observed value to return")
	}
	return m.min
}

func CountNonZero(x [8]TileId) (c int) {
	for i := range x {
		if x[i] != 0 {
			c++ // what's this :)
		}
	}
	return
}
