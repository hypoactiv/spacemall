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
	for i := 0; i < 800; i++ {
		l := l.JustOffset(rand.Intn(50)-10, rand.Intn(50)-10)
		w.Spawn(NewConwayCell(l))
	}
	for i := 0; i < 50000; i++ {
		w.Think()
	}
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
}
