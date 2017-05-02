package entity

import (
	"fmt"
	"jds/game"
	"jds/game/world"
	"math"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkRandomWalker(b *testing.B) {
	b.StopTimer()
	N := 192 * 108
	rtN := int(math.Sqrt(float64(N)))
	w := world.NewWorld(0)
	E := make([]RandomWalker, N)
	cursor := game.Location{}
	w.DrawBox(cursor.JustOffset(-2, -2), cursor.JustOffset(2*rtN+2, 2*rtN+2))
	for i := 0; i < N; i++ {
		E[i].l = cursor.JustOffset(rand.Intn(2*rtN), rand.Intn(2*rtN))
		if eid := w.Spawn(&E[i]); eid == world.ENTITYID_INVALID {
			i--
			continue
		}
	}
	b.StartTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		w.Think()
	}
	fmt.Println("executed", w.ActionCount, "actions in", time.Since(start))
	fmt.Println(float64(w.ActionCount) / time.Since(start).Seconds() / 1000000)
}

func TestRandomWalker(t *testing.T) {
	N := 1000
	rtN := int(math.Sqrt(float64(N)))
	w := world.NewWorld(0)
	E := make([]RandomWalker, N)
	cursor := game.Location{}
	w.DrawBox(cursor.JustOffset(-2, -2), cursor.JustOffset(2*rtN+2, 2*rtN+2))
	for i := 0; i < N; i++ {
		E[i].l = cursor.JustOffset(rand.Intn(2*rtN), rand.Intn(2*rtN))
		if eid := w.Spawn(&E[i]); eid == world.ENTITYID_INVALID {
			i--
			continue
		}
	}
	for i := 0; i < 100; i++ {
		w.Think()
	}
	if numSteps == 0 {
		t.Error("no steps taken")
	}
}
