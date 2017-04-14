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
	N := 100
	w := world.NewWorld(0)
	E := make([]RandomWalker, N)
	cursor := game.Location{}
	w.DrawBox(cursor.JustOffset(-2, -2), cursor.JustOffset(N+2, 20))
	for i := 0; i < N; i++ {
		E[i].l = cursor
		if eid := w.Spawn(&E[i]); eid == world.ENTITYID_INVALID {
			t.Error("unable to spawn", i)
		}
		cursor = cursor.JustStep(game.RIGHT)
	}
	for i := 0; i < 10000; i++ {
		w.Think()
	}
	for i := 0; i < N; i++ {
		if E[i].spawned != 1 {
			t.Error("wrong event count", i, E[i].spawned)
		}
		//t.Log(E[i].touched, E[i].hitWall)
	}
	if numSteps == 0 {
		t.Error("no steps taken")
	}
}
