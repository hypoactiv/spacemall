package entity

import (
	"jds/game"
	"jds/game/world"
	"math/rand"
	"testing"
)

func BenchmarkConwayCell(b *testing.B) {
	b.StopTimer()
	w := world.NewWorld(0)
	l := game.Location{}
	for i := 0; i < 100; i++ {
		l := l.JustOffset(rand.Intn(20)-10, rand.Intn(20)-10)
		w.Spawn(NewConwayCell(l))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		w.Think()
	}
}
