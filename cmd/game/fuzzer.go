package main

import (
	"fmt"
	"jds/game"
	"math/rand"
	"runtime/debug"
	"time"
)

// Returns random on-screen location
func (te *TileEngine) RandomLocation(scale int) game.Location {
	return te.ScreenToWorld(rand.Intn(WIDTH/scale)*scale, rand.Intn(HEIGHT/scale)*scale)
}

func (te *TileEngine) Fuzz(stopAt int) (err interface{}) {
	fmt.Println("fuzzer running...")
	ops := 0
	lastOps := 0
	lastStatus := time.Now()
	for ops != stopAt && !exit {
		if time.Since(lastStatus) > 10*time.Second {
			fmt.Printf("%.2f fuzz op/sec %d adds %d deletes %s\n",
				float64(ops-lastOps)/time.Since(lastStatus).Seconds(),
				te.w.AddOps,
				te.w.DeleteOps)
			lastStatus = time.Now()
			lastOps = ops
			//te.Render()
		}
		func() {
			defer func() {
				err = recover()
				if err != nil {
					fmt.Println("fuzzer exiting with error")
					fmt.Println(err)
					debug.PrintStack()
				}
			}()
			ops++
			//if ops%1000 == 0 {
			//	te.w.FsckWallTree()
			//}
			l1 := te.RandomLocation(90)
			l2 := l1.JustOffset(rand.Intn(5)*30, rand.Intn(5)*30)
			var t Tool
			switch rand.Intn(5) {
			//case 0:
			// Draw two points
			//t = NewPointTool(te.w)
			//case 1:
			//Draw line
			//t = NewLineTool(te.w)
			case 2:
				// Draw box
				t = NewBoxTool(te.w)
			default:
				// Delete
				t = NewDeleteTool(te.w)
			case 0, 4:
				t = NewPlaceDoorTool(te.w)
			}
			te.background.UpdateBulk(t.Click(l1))
			te.background.UpdateBulk(t.Click(l2))
		}()
		if err != nil {
			break
		}
	}
	exit = false
	fmt.Println("fuzzer exiting after", ops, "operations")
	return
}
