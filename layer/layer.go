package layer

import (
	"jds/game"
	"jds/game/patterns"
)

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
