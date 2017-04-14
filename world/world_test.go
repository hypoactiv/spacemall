package world

import (
	"fmt"
	"jds/game"
	"testing"
)

func BenchmarkNestedRooms(b *testing.B) {
	fmt.Println(b.N)
	b.Log(b.N)
	for j := 0; j < b.N; j++ {
		w := NewWorld(0)
		ul := game.Location{}
		N := 32
		lr := ul.JustOffset(N, N)
		for i := 0; i < N/4; i++ {
			for loc := range game.Box(ul, lr) {
				w.SetWall(loc)
			}
			ul = ul.JustOffset(2, 2)
			lr = lr.JustOffset(-2, -2)
		}
		w.Walls.Discard()
		w.RoomIds.Discard()
		w.ForcedFlags.Discard()
	}
}

func TestNestedRooms(t *testing.T) {
	w := NewWorld(STRICT_ALL)
	N := 16
	ul := game.Location{}
	lr := ul.JustOffset(N, N)
	for i := 0; i < N/4; i++ {
		t.Logf("nest: %d %s %s", i, ul, lr)
		for loc := range game.Box(ul, lr) {
			w.SetWall(loc)
		}
		ul = ul.JustOffset(2, 2)
		lr = lr.JustOffset(-2, -2)
	}
	for k, v := range w.Rooms {
		t.Log(k, v.Area)
	}
}

func seqmap(n int) (m map[int]struct{}) {
	m = make(map[int]struct{})
	for i := 0; i < n; i++ {
		m[i] = struct{}{}
	}
	return
}

func TestLatticeRoom(t *testing.T) {
	w := NewWorld(STRICT_ALL)
	N := 4 // Number of cells along square lattice edge
	s := 5 // Lattice cell size
	for i := range seqmap(N) {
		for j := range seqmap(N) {
			ul := game.Location{}.JustOffset(i*s, j*s)
			t.Log(ul)
			lr := ul.JustOffset(s, s)
			for loc := range game.Box(ul, lr) {
				w.SetWall(loc)
			}
		}
	}
	for k, v := range w.Rooms {
		if v.Area != int((s-1)*(s-1)) {
			t.Errorf("rid: %d incorrect area", k)
		}
		t.Logf("rid: %d area: %d", k, v.Area)
	}
	if len(w.Rooms) != int(N*N) {
		t.Errorf("incorrect number of rooms")
	}
}

func BenchmarkLatticeRoom(b *testing.B) {
	fmt.Println(b.N)
	for j := 0; j < b.N; j++ {
		w := NewWorld(0)
		N := 4 // Number of cells along square lattice edge
		s := 5 // Lattice cell size
		for i := 0; i < N; i++ {
			for j := 0; j < N; j++ {
				ul := game.Location{}.JustOffset(i*s, j*s)
				lr := ul.JustOffset(s, s)
				for loc := range game.Box(ul, lr) {
					w.SetWall(loc)
				}
			}
		}
		w.Walls.Discard()
		w.RoomIds.Discard()
		w.ForcedFlags.Discard()
	}
}
