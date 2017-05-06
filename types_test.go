package game

import (
	"math/rand"
	"testing"
	"unsafe"
)

func TestLocationStep(t *testing.T) {
	l := Location{}
	for d, ln := range l.Neighborhood() {
		if ln2, _, _ := l.Step(Direction(d)); ln2 != ln {
			t.Errorf("step inconsistent")
		}
	}
}

func TestRowMask(t *testing.T) {
	N := 1000
	rm := NewRowMask(N)
	truth := make([]bool, N)
	for i := range truth {
		if i >= N/2 {
			truth[i] = true
		}
		rm.Append(truth[i])
	}
	t.Log(rm.mask)
	for i, actual := range truth {
		if got, _ := rm.Mask(i); got != actual {
			t.Errorf("inconsistent %d %v %v", i, got, actual)
		}
	}
}

func TestRowMaskDist(t *testing.T) {
	N := 1000
	rm := NewRowMask(N)
	truth := make([]bool, N)
	for i := range truth {
		if rand.Intn(30) == 1 {
			truth[i] = true
		}
		rm.Append(truth[i])
	}
	i := 0
	for i < rm.Width() {
		_, skip := rm.Mask(i)
		for j := 0; j < skip; j++ {
			if truth[i] != truth[i+j] {
				t.Errorf("unexpected change")
			}
		}
		i += skip
	}
}

func TestRowMaskRandom(t *testing.T) {
	N := 1000
	rm := NewRowMask(N)
	truth := make([]bool, N)
	for i := range truth {
		if rand.Intn(2) == 1 {
			truth[i] = true
		}
		rm.Append(truth[i])
	}
	for i, actual := range truth {
		if got, _ := rm.Mask(i); got != actual {
			t.Errorf("inconsistent %d", i)
		}
	}
}

func BenchmarkRowMaskRandom(b *testing.B) {
	b.StopTimer()
	N := 1000
	rm := NewRowMask(N)
	truth := make([]bool, N)
	for i := range truth {
		if rand.Intn(2) == 1 {
			truth[i] = true
		}
		rm.Append(truth[i])
	}
	b.StartTimer()
	for j := 0; j < b.N/N; j++ {
		for i, _ := range truth {
			rm.Mask(i)
		}
	}
}

func TestTowards(t *testing.T) {
	a := Location{}
	b := a.JustOffset(rand.Intn(100), rand.Intn(100))
	t.Log(a, b)
	for a.MaxDistance(b) > 0 {
		a = a.JustStep(a.Towards(b))
		t.Log(a.MaxDistance(b), a)
	}
}

func TestTypeSizes(t *testing.T) {
	t.Log("Location", unsafe.Sizeof(Location{}))
	t.Log("Direction", unsafe.Sizeof(Direction(0)))
}
