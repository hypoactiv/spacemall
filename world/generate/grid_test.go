package generate

import (
	"jds/game"
	"testing"
)

func BenchmarkInterior(b *testing.B) {
	w := NewGridWorld(10, 10) //NewWorld(0)
	i := 0
	for {
		for _, v := range w.Rooms {
			if i >= b.N {
				return
			}
			i++
			count := 0
			v.Interior(func(rm *game.RowMask) bool {
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
	}
}
