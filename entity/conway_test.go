package entity

import (
	"jds/game"
	"jds/game/world"
	"math/rand"
	"testing"
	"time"
)

func TestConwayCell(t *testing.T) {
	rand.Seed(time.Now().Unix())
	w := world.NewWorld(0)
	l := game.Location{}
	for i := 0; i < 10000; i++ {
		l := l.JustOffset(rand.Intn(500)-10, rand.Intn(50)-10)
		w.Spawn(NewConwayCell(l))
	}
	start := time.Now()
	ticks := 0
	for time.Since(start) < 10*time.Second && ticks < 1000 {
		w.Think()
		ticks++
	}
	t.Log("Avg workers per tick:", float32(w.ThinkStats.Workers)/float32(ticks))
	t.Log("Avg Actions per worker:", float32(w.ThinkStats.Actions)/float32(w.ThinkStats.Workers))
	t.Log("Avg time per Think:", w.ThinkStats.Elapsed/time.Duration(ticks))
	t.Log("Avg Actions per second:", float64(w.ThinkStats.Actions)/w.ThinkStats.Elapsed.Seconds())
	w.ThinkStats.Actions = 0
	w.ThinkStats.Workers = 0
	w.ThinkStats.Elapsed = 0
	w.Discard()
}

func BenchmarkConwayCell(b *testing.B) {
	b.StopTimer()
	w := world.NewWorld(0)
	l := game.Location{}
	for i := 0; i < 800; i++ {
		l := l.JustOffset(rand.Intn(50)-10, rand.Intn(50)-10)
		w.Spawn(NewConwayCell(l))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		w.Think()
	}
	b.StopTimer()
	w.Discard()
}
