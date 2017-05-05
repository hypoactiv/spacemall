package path

import (
	"jds/game"
	"jds/game/world"
	"jds/game/world/generate"
	"math/rand"
	"sync"
	"testing"
)

func BenchmarkZigZagRoute(b *testing.B) {
	defer b.Logf("Done N:%d init:%d alloc:%d, release:%d leak:%d", b.N, nInits, nAlloc, nRelease, nAlloc-nRelease)
	b.StopTimer()
	w := world.NewWorld(0)
	N := 400
	spacing := 3
	ul := game.Location{}
	lr := ul.JustOffset(N, N)
	for loc := range game.Box(ul, lr) {
		w.SetWall(loc)
	}
	ul = ul.JustOffset(1, 1)
	for i := 0; i < N-2; i++ {
		for j := 1; j <= N/spacing; j++ {
			w.SetWall(ul.JustOffset(i+((j+1)%2), j*spacing))
		}
	}
	lr = ul.JustOffset(N-2, N-2)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		NewRoute(w, ul, lr)
	}
}

func TestZigZagRoute(t *testing.T) {
	w := world.NewWorld(0)
	N := 400
	spacing := 10
	ul := game.Location{}
	lr := ul.JustOffset(N, N)
	for loc := range game.Box(ul, lr) {
		w.SetWall(loc)
	}
	ul = ul.JustOffset(1, 1)
	for i := 0; i < N-2; i++ {
		for j := 1; j <= N/spacing; j++ {
			w.SetWall(ul.JustOffset(i+((j+1)%2), j*spacing))
		}
	}
	lr = ul.JustOffset(N-2, N-2)
	r := NewRoute(w, ul, lr)
	j := 0
	for _, rs := range r {
		for i := uint(0); i < rs.Length; i++ {
			ul = ul.JustStep(rs.D)
			j++
			if w.Walls.Get(ul) != 0 {
				t.Error("route goes through wall")
			}
		}
	}
	t.Log("received path of length", j)
	if ul != lr {
		t.Error("didn't arrive at destination")
	} else {
		t.Log("reached", lr, "in", j, "steps")
	}
}

func BenchmarkWalkAroundRoom(b *testing.B) {
	b.StopTimer()
	w := world.NewWorld(0)
	N := 50
	ul := game.Location{}
	lr := ul.JustOffset(N, N)
	w.DrawBox(ul, lr)
	w.DrawBox(ul.JustOffset(N/4, N/4), ul.JustOffset(3*N/4, 3*N/4))
	ul = ul.JustOffset(1, 1)
	lr = ul.JustOffset(N-2, N-2)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		NewRoute(w, ul, lr)
	}
}

func TestSimpleWalk(t *testing.T) {
	w := world.NewWorld(0)
	N := 64
	ul := game.Location{}
	lr := ul.JustOffset(N, N)
	w.DrawBox(ul, lr)
	ul = ul.JustOffset(1, 1)
	lr = ul.JustOffset(1, N-2)
	r := NewRoute(w, ul, lr)
	steps := 0
	for _, rs := range r {
		for i := uint(0); i < rs.Length; i++ {
			steps++
			ul = ul.JustStep(rs.D)
			if w.Walls.Get(ul) != 0 {
				t.Error("route goes through wall")
			}
		}
	}
	if ul != lr {
		t.Errorf("%v != %v steps:%d", ul, lr, steps)
		t.Error("didn't arrive at destination")
		panic("asdf")
	}
}

func TestRandomWalkAroundRoom(t *testing.T) {
	w := world.NewWorld(0)
	N := 64
	ul := game.Location{}
	lr := ul.JustOffset(N, N)
	// draw a large box with a smaller box inside it
	w.DrawBox(ul, lr)
	w.DrawBox(ul.JustOffset(N/4, N/4), ul.JustOffset(3*N/4, 3*N/4))
	for j := 0; j < 10000; j++ {
		ul = game.Location{}
		ul = ul.JustOffset(rand.Intn(N-1)+1, rand.Intn(N-2)+1)
		lr = ul.JustOffset(rand.Intn(N-1)+1, rand.Intn(N-2)+1)
		if ulrid := w.RoomIds.Get(ul); ulrid == 0 || ulrid != w.RoomIds.Get(lr) {
			// only route inside one room for now
			continue
		}
		r := NewRoute(w, ul, lr)
		steps := 0
		for _, rs := range r {
			for i := uint(0); i < rs.Length; i++ {
				steps++
				ul = ul.JustStep(rs.D)
				if w.Walls.Get(ul) != 0 {
					t.Error("route goes through wall")
				}
			}
		}
		if ul != lr {
			t.Errorf("pass %d: %v != %v steps:%d", j, ul, lr, steps)
			t.Error("didn't arrive at destination")
			panic("asdf")
		}
	}
}

func BenchmarkRandomWalkAroundRoom(b *testing.B) {
	b.StopTimer()
	w := world.NewWorld(0)
	N := 64
	ul := game.Location{}
	lr := ul.JustOffset(N, N)
	w.DrawBox(ul, lr)
	for i := 0; i < 10; i++ {
		w.SetWall(ul.JustOffset(rand.Intn(N-4)+1, rand.Intn(N-4)+1))
	}
	b.StartTimer()
	for j := 0; j < b.N; j++ {
		ul = game.Location{}
		lr = ul.JustOffset(rand.Intn(N-4)+1, rand.Intn(N-4)+1)
		ul = ul.JustOffset(rand.Intn(N-4)+1, rand.Intn(N-4)+1)
		NewRoute(w, ul, lr)
	}
}

func randomRoomLoc(r *world.Room) (loc game.Location) {
	index := rand.Intn(r.Area)
	c := 0
	loc = game.Location{}
	r.Interior(func(rm *game.RowMask, unused []game.TileId) bool {
		for i := 0; i < rm.Width(); i++ {
			m, next := rm.Mask(i)
			if m == false {
				i += next - 1
				continue
			}
			if c == index {
				loc, _, _ = rm.Left.FarStep(game.RIGHT, i)
				return false
			}
			c++
		}
		return true
	})
	return
}

func TestGridWorldWalk(t *testing.T) {
	w := generate.NewGridWorld(10, 10)
	m := game.Min{}
	// find largest room
	for _, r := range w.Rooms {
		m.Observe(r, -r.Area)
	}
	largest := m.Argmin().(*world.Room)
	dest := randomRoomLoc(largest)
	wg := sync.WaitGroup{}
	N := 4
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			largest.Interior(func(rm *game.RowMask, unused []game.TileId) bool {
				for i := 0; i < rm.Width(); i++ {
					m, next := rm.Mask(i)
					if m == false {
						i += next - 1
						continue
					}
					cursor, _, _ := rm.Left.FarStep(game.RIGHT, i)
					NewRoute(w, cursor, dest)
				}
				return true
			})
		}()
	}
	wg.Wait()
}
