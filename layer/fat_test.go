package layer

import (
	"jds/game"
	"math/rand"
	"testing"
)

func TestSetAndGet(t *testing.T) {
	cursor := game.Location{}
	l := NewLayer()
	for i := game.TileId(0); i < 100; i++ {
		l.Set(cursor, i)
		cursor, _, _ = cursor.Right()
	}
	cursor = game.Location{}
	for i := game.TileId(0); i < 100; i++ {
		if actual := l.Get(cursor); actual != i {
			t.Errorf("read inconsistent i=%d got=%d want=%d\n", i, actual, i)
		}
		cursor, _, _ = cursor.Right()
	}
}

func TestNbdPtrs(t *testing.T) {
	N := 100000
	l := NewLayer()
	defer l.Discard()
	for j := 0; j < 5; j++ {
		cursor := game.Location{}
		for i := 0; i < N; i++ {
			l.Set(cursor, game.TileId(i*j))
			cursor, _, _ = cursor.Step(game.Direction(rand.Intn(8)))
		}
	}
	for bid, b := range l.bs {
		for d, nbid := range bid.Neighbors() {
			if b.N[d] != l.bs[nbid] {
				panic("neighbor pointers inconsistent")
			}
		}
	}
}

func TestLook(t *testing.T) {
	l := NewLayer()
	defer l.Discard()
	loc := game.Location{}
	for d, n := range loc.Neighborhood() {
		l.Set(n, game.TileId(d))
	}
	cursor := game.Location{}
	sc := NewStackCursor(cursor)
	if sc.Cursor() != cursor {
		t.Errorf("%+v %+v", sc.Cursor(), cursor)
		panic("cursor inconsistent")
	}
	li := sc.Add(l)
	local := sc.Look(li)
	for d, v := range local {
		if v != game.TileId(d) {
			panic("look inconsistent")
		}
	}
}

func TestOffsetGet(t *testing.T) {
	N := 10000
	l := NewLayer()
	defer l.Discard()
	for j := 0; j < 5; j++ {
		cursor := game.Location{}
		for i := 0; i < N; i++ {
			l.Set(cursor, game.TileId(i*j))
			cursor, _, _ = cursor.Step(game.Direction(rand.Intn(8)))
		}
	}
	cursor := game.Location{}
	sc := NewStackCursor(cursor)
	li := sc.Add(l)
	for i := 0; i < N; i++ {
		dx, dy := sc.Cursor().SmallDistance(cursor)
		got := sc.OffsetGet(li, dx, dy)
		want := l.Get(cursor)
		if got != want {
			t.Errorf("walk read inconsistent. got:%d want:%d. i=%d\n", got, want, i)
			panic("stop")
		}
		cursor = cursor.JustStep(game.Direction(rand.Intn(8)))
	}
}

func TestReadWalk(t *testing.T) {
	N := 10000
	l := NewLayer()
	defer l.Discard()
	for j := 0; j < 5; j++ {
		cursor := game.Location{}
		for i := 0; i < N; i++ {
			l.Set(cursor, game.TileId(i*j))
			cursor, _, _ = cursor.Step(game.Direction(rand.Intn(8)))
		}
	}
	cursor := game.Location{}
	sc := NewStackCursor(cursor)
	if sc.Cursor() != cursor {
		t.Errorf("%+v %+v", sc.Cursor(), cursor)
		panic("cursor inconsistent")
	}
	li := sc.Add(l)
	for i := 0; i < N; i++ {
		p := sc.Look(li)
		for d, v := range p {
			testLoc, _, _ := sc.Cursor().Step(game.Direction(d))
			if actual := l.Get(testLoc); actual != v {
				t.Errorf("%s %v %p %p", testLoc, testLoc.BlockId, l.bs[testLoc.BlockId], sc.b[li])
				t.Errorf("%d", sc.b[li].tiles[testLoc.Y][testLoc.X])
				t.Errorf("walk read inconsistent. got:%d want:%d. d=%s i=%d\n", v, actual, game.Direction(d), i)
				panic("stop")
			}
		}
		sc.Step(game.Direction(rand.Intn(8)))
	}
}

func BenchmarkLayerStepGet(b *testing.B) {
	b.StopTimer()
	l := NewLayer()
	defer l.Discard()
	for j := 0; j < 5; j++ {
		cursor := game.Location{}
		for i := 0; i < 1000; i++ {
			l.Set(cursor, game.TileId(i*j))
			cursor, _, _ = cursor.Step(game.Direction(rand.Intn(8)))
		}
	}
	cursor := game.Location{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		l.Get(cursor)
		cursor = cursor.JustStep(game.Direction(i % 8))
	}
}

func BenchmarkStackCursorStepGet(b *testing.B) {
	b.StopTimer()
	l := NewLayer()
	defer l.Discard()
	for j := 0; j < 5; j++ {
		cursor := game.Location{}
		for i := 0; i < 1000; i++ {
			l.Set(cursor, game.TileId(i*j))
			cursor, _, _ = cursor.Step(game.Direction(rand.Intn(8)))
		}
	}
	cursor := game.Location{}
	sc := NewStackCursor(cursor)
	li := sc.Add(l)
	steps := make([]game.Direction, 8)
	for i := 0; i < 8; i++ {
		steps[i] = game.Direction(rand.Intn(8))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sc.Get(li)
		sc.Step(game.Direction(i % 8))
	}
}

func BenchmarkStackCursorStepLook(b *testing.B) {
	b.StopTimer()
	l := NewLayer()
	defer l.Discard()
	for j := 0; j < 5; j++ {
		cursor := game.Location{}
		for i := 0; i < 1000; i++ {
			l.Set(cursor, game.TileId(i*j))
			cursor, _, _ = cursor.Step(game.Direction(rand.Intn(8)))
		}
	}
	cursor := game.Location{}
	sc := NewStackCursor(cursor)
	li := sc.Add(l)
	steps := make([]game.Direction, 8)
	for i := 0; i < 8; i++ {
		steps[i] = game.Direction(rand.Intn(8))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sc.Look(li)
		sc.Step(game.Direction(i % 8))
	}
}

func TestFatWriteWalk(t *testing.T) {
	N := 100000
	l := NewLayer()
	start := game.Location{}
	sc := NewStackCursor(start)
	li := sc.Add(l)
	for i := game.TileId(0); int(i) < N; i++ {
		sc.Set(li, i)
		if actual := l.Get(sc.Cursor()); actual != i {
			t.Errorf("readback inconsistent wanted:%d got:%d", i, actual)
		}
		sc.Step(game.Direction(rand.Intn(8)))
	}
}

// TODO this only works a very small amount of functionality (check other directions, for example)
func TestBitScan(t *testing.T) {
	l := NewLayer()
	c := game.Location{}
	for i := uint(0); i < 31; i++ {
		l.Set(c.JustOffset(int(i), 0), (1 << i))
	}
	sc := NewStackCursor(c)
	li := sc.Add(l)
	for j := 0; j < 30; j++ {
		for i := uint(j); i < 31; i++ {
			scanDist := sc.ScanBit(li, game.RIGHT, -1, i)
			if scanDist < 0 {
				t.Errorf("negative distance")
				panic("asdf")
			}
			if scanDist != int(i)-j {
				t.Errorf("inconsistent got:%d wanted:%d j:%d", scanDist, int(i)-j, int(j))
			}
		}
		sc.Step(game.RIGHT)
	}
}

func TestCollectRowMask(t *testing.T) {
	N := 1000
	l := NewLayer()
	cursor := game.Location{}
	rm := game.NewRowMask(N, cursor)
	for i := 0; i < N; i++ {
		if i%10 == 0 {
			l.Set(cursor, game.TileId(i))
		}
		if i%20 == 0 {
			rm.Append(true)
		} else {
			rm.Append(false)
		}
		cursor, _, _ = cursor.Right()
	}
	cursor = game.Location{}
	j := 0
	v, d := l.CollectRowMask(rm)
	for i := 0; i < N; i++ {
		paint, _ := rm.Mask(i)
		if actual := l.Get(cursor); paint == true && actual != 0 {
			if d[j] != i || v[j] != actual {
				t.Errorf("inconsistent %d", i)
			}
			j++
		}
		cursor, _, _ = cursor.Right()
	}
}

func TestSetRowMask(t *testing.T) {
	N := 100
	cursor := game.Location{}
	rm := game.NewRowMask(N, cursor)
	truth := make([]bool, N)
	for i := range truth {
		if rand.Intn(2) == 1 {
			truth[i] = true
		}
		rm.Append(truth[i])
	}
	l := NewLayer()
	l.SetRowMask(rm, 1, nil)
	for i := 0; i < N; i++ {
		actual := l.Get(cursor) == 1
		if truth[i] != actual {
			t.Errorf("inconsistent %d", i)
		}
		cursor, _, _ = cursor.Right()
	}
}

func TestGetRow(t *testing.T) {
	N := 500
	l := NewLayer()
	defer l.Discard()
	cursor := game.Location{}
	sc := NewStackCursor(cursor)
	li := sc.Add(l)
	for i := 0; i < N; i++ {
		l.Set(cursor, game.TileId(i))
		cursor, _, _ = cursor.Right()
	}
	row := make([]game.TileId, N)
	for j := 0; j < N; j++ {
		sc.GetRow(li, row)
		for i, v := range row {
			if int(v) != i+j && i+j < N {
				t.Errorf("inconsistent %d %d %d", i, j, v)
				panic("asdf")
			}
		}
		sc.Step(game.RIGHT)
	}
}

func TestObstructed(t *testing.T) {
	l := NewLayer()
	defer l.Discard()
	cursor := game.Location{}
	l.Set(cursor.JustOffset(10, 0), 1)
	target := cursor.JustOffset(20, 0)
	sc := NewStackCursor(cursor)
	li := sc.Add(l)
	if sc.Obstructed(li, target) != true {
		t.Error("should be obstructed")
	}
}

func BenchmarkObstructed(b *testing.B) {
	b.StopTimer()
	l := NewLayer()
	defer l.Discard()
	cursor := game.Location{}
	l.Set(cursor.JustOffset(10, 0), 1)
	target := cursor.JustOffset(32, 0)
	sc := NewStackCursor(cursor)
	li := sc.Add(l)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sc.Obstructed(li, target)
	}
}

/* Template:
func Test (t *testing.T) {
	l := NewLayer()
	defer l.Discard()
	cursor := game.Location{}
	sc := NewStackCursor(cursor)
	li := sc.Add(l)
}
*/
