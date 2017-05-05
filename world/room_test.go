package world

import (
	"jds/game"
	"sync"
	"testing"
)

func TestInterior(t *testing.T) {
	w := NewWorld(0)
	l := game.Location{}
	w.DrawBox(l, l.JustOffset(100, 100))
	N := 4
	wg := sync.WaitGroup{}
	wg.Add(N)
	for j := 0; j < N; j++ {
		go func() {
			defer wg.Done()
			for _, v := range w.Rooms {
				count := 0
				v.Interior(func(rm *game.RowMask, unused []game.TileId) bool {
					for i := 0; i < rm.Width(); i++ {
						if m, len := rm.Mask(i); m == false {
							i += len - 1
							continue
						}
						count++
					}
					return true
				})
				if v.Area != count {
					panic("area wrong")
				}
			}
		}()
	}
	wg.Wait()
}
