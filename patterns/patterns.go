package patterns

import "jds/game"

type Pattern struct {
	P []game.TileId
	W int
}

func (p *Pattern) H() int {
	return len(p.P) / p.W
}

// Returns a copy of Pattern p with all non-zero entries replaced with v
func (p Pattern) Remap(v game.TileId) (q Pattern) {
	q.P = make([]game.TileId, len(p.P))
	q.W = p.W
	for i, w := range p.P {
		if w != 0 {
			q.P[i] = v
		}
	}
	return
}

var Zero4x3 = Pattern{
	P: []game.TileId{
		0, 0, 0,
		0, 0, 0,
		0, 0, 0,
		0, 0, 0,
	},
	W: 3,
}

var Door = Pattern{
	P: []game.TileId{
		0, 1, 0,
		0, 1, 0,
		0, 1, 0,
		0, 1, 0,
	},
	W: 3,
}

var DoorId = Pattern{
	P: []game.TileId{
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
	},
	W: 3,
}

var Block = Pattern{
	P: []game.TileId{
		1, 1,
		1, 1,
	},
	W: 2,
}
