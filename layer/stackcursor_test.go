package layer

import (
	"jds/game"
	"math/rand"
	"testing"
)

func randomlocation(n int) game.Location {
	return game.Location{}.JustOffset(rand.Intn(n), rand.Intn(n))
}

func BenchmarkSetBitGetBit(b *testing.B) {
	l := NewLayer()
	defer l.Discard()
	sc := NewStackCursor(game.Location{})
	li := sc.Add(l)
	for i := 0; i < b.N; i++ {
		sc.MoveTo(game.Location{X: int8(i % (game.BLOCK_SIZE * 8))})
		v := sc.GetBit(li, uint(i%32))
		sc.SetBit(li, uint(i%32), !v)
	}
}

func TestSetBitGetBit(t *testing.T) {
	N := 100
	l := NewLayer()
	defer l.Discard()
	sc := NewStackCursor(game.Location{})
	li := sc.Add(l)
	for i := 0; i < N; i++ {
		c := randomlocation(N)
		sc.MoveTo(c)
		b := uint(rand.Intn(32))
		v := sc.GetBit(li, b)
		sc.SetBit(li, b, !v)
		// now expect bit b at location c to have value !v (v inverse)
		// verify this with GetBit
		if sc.GetBit(li, b) != !v {
			t.Errorf("failed to toggle bit %b at %v", b, c)
		}
		// verify this with Get
		if (l.Get(c)&(1<<b) != 0) != !v {
			t.Errorf("incorrect value read from %v", c)
		}
	}
}

func TestDirectedGetSet(t *testing.T) {
	l := NewLayer()
	defer l.Discard()
	sc := NewStackCursor(game.Location{})
	li := sc.Add(l)
	for i := game.Direction(0); i < 8; i++ {
		sc.DirectedSet(li, i, game.TileId(i))
	}
	for i := game.Direction(0); i < 8; i++ {
		if got, want := sc.DirectedGet(li, i), game.TileId(i); got != want {
			t.Error("want", want, "got", got)
		}
	}
}
