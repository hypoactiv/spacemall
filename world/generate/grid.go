package generate

import (
	"jds/game"
	"jds/game/layer"
	"jds/game/world"
	"math/rand"
)

func NewGridWorld(width, height int) (w *world.World) {
	w = world.NewWorld(0)
	floorplan := layer.NewLayer()
	cursor := game.Location{}.JustOffset(10, 10)
	cellX := make([]int, width+1)
	cellY := make([]int, height+1)
	for j := 0; j < height; j++ {
		for i := 0; i < width; i++ {
			n := rand.Intn(6) + 2
			if j%3 == 0 || i%4 == 0 {
				n = 1
			}
			floorplan.Set(cursor.JustOffset(i, j), game.TileId(n))
		}
	}
	for i := 1; i <= width; i++ {
		cellX[i] = cellX[i-1] + rand.Intn(8) + 8
	}
	for i := 1; i <= height; i++ {
		cellY[i] = cellY[i-1] + rand.Intn(6) + 12
	}
	//w.DrawBox(
	//	cursor,
	//	cursor.JustOffset(cellX[width], cellY[height]))
	for i := 0; i <= width; i++ {
		for j := 0; j <= height; j++ {
			this := floorplan.Get(cursor.JustOffset(i, j))
			left := floorplan.Get(cursor.JustOffset(i-1, j))
			above := floorplan.Get(cursor.JustOffset(i, j-1))
			if this != left && j < height {
				// left type different, draw wall
				w.DrawLine(
					cursor.JustOffset(cellX[i], cellY[j]),
					cursor.JustOffset(cellX[i], cellY[j+1]))
			}
			if this != above && i < width {
				// above type different, draw wall
				w.DrawLine(
					cursor.JustOffset(cellX[i], cellY[j]),
					cursor.JustOffset(cellX[i+1], cellY[j]))
			}
		}
	}
	/*numEntities := cellX[width] * cellY[height] / 25
	for i := 0; i < numEntities; i++ {
		var rl game.Location
	retry:
		for {
			rl = cursor.JustOffset(rand.Intn(cellX[width]), rand.Intn(cellY[height]))
			// look for a location in a room
			if w.RoomIds.Get(rl) != 0 {
				break
			}
		}
		if eid := w.Spawn(entity.NewRandomWalker(rl)); eid == world.ENTITYID_INVALID {
			// spawn failed for some reason -- tile occupied?
			goto retry
		}
	}*/
	return
}
