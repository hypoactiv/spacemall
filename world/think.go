package world

import (
	"sync"
	"time"
)

func (w *World) Think() {
	start := time.Now()
	// increment time
	w.ticks++
	// Buffer ScheduledActions for w.ticks from actionSchedule
	taTmp := AllocateAA(w.ticks)
	for w.actionSchedule.Len() > 0 {
		if w.actionSchedule.PeekTick() > w.ticks {
			break
		}
		taTmp.AddAction(w.actionSchedule.Next())
	}
	taTmp.Close()
	w.process(taTmp, false)
	ReleaseAA(taTmp)
	// Swap buffer and execute work units
	w.workUnits[WU_BUFFER], w.workUnits[WU_EXECUTE] = w.workUnits[WU_EXECUTE], w.workUnits[WU_BUFFER]
	// w.workUnits[WU_EXECUTE] now contains workUnits to be processed during tick
	// w.ticks, sorted by BlockId X value
	wuExe := w.workUnits[WU_EXECUTE]
	wuLen := len(wuExe)
	if wuLen == 0 {
		// No work units -- nothing to do
		return
	}
	// Worker closure
	wgWorkers := sync.WaitGroup{}
	worker := func(wuRunStart, wuRunEnd int, aa *ActionAccumulator) {
		// Process workUnits between wuRunStart and wuRunEnd, inclusive
		for i := wuRunStart; i <= wuRunEnd; i++ {
			if wuExe[i].done {
				panic("tried to execute completed workUnit")
			}
			for _, action := range wuExe[i].Actions {
				action(aa)
			}
			wuExe[i].done = true
		}
		// assigned workUnits are done, now unlock
		if wuRunStart > 0 {
			wuExe[wuRunStart-1].locked = false
		}
		for i := wuRunStart; i <= wuRunEnd; i++ {
			wuExe[i].locked = false
		}
		if wuRunEnd < wuLen-1 {
			wuExe[wuRunEnd+1].locked = false
		}
		aa.Close()
		wgWorkers.Done()
	}
	// Collect the ActionAccumulators given to the workers here
	workerAAs := make([]*ActionAccumulator, 0)
	// Divide the workUnits into groups of a reasonable size, and launch workers
	// to process the workUnits
	moreWork := true
	i := -1
	for moreWork { // Loop until all workUnits are done
		moreWork = false
		// Find the start of a run of workUnits
		wuRunStart := -1
		wuRunEnd := 0
		for j := 0; j < wuLen; j++ { //i, wu := range wuExe {
			i = ((i + 1) % wuLen)
			wu := &wuExe[i]
			if wu.done {
				continue
			}
			moreWork = true // at least one workUnit is not done
			if wu.locked {
				continue
			}
			if wu.done { // must re-check done flag after checking locked, as status may have changed from above check
				continue
			}
			if len(wu.Actions) == 0 {
				wu.done = true
				continue
			}
			// Can wuExe[i] be processed now? Must be able to lock left and right neighboring columns
			if i > 0 && wuExe[i-1].locked == true {
				// No, workUnit for left neighboring column is locked
				continue
			}
			if i < wuLen-1 && wuExe[i+1].locked == true {
				// No, workUnit for right neighboring column is locked
				continue
			}
			// This workUnit can be processed, start a new run
			//fmt.Println("ok start", i, wu.done, wu.locked, w.ticks)
			wuRunStart = i
			break
		}
		if wuRunStart != -1 {
			actionCount := len(wuExe[wuRunStart].Actions)
			// Found the start of a run of workUnits, now find its end
			// Initial condition:
			// wuExe[wuRunStart] not done or locked,
			// and wuExe[wuRunStart+1] not locked (if it exists)
			// wuRunEnd == wuRunStart
			for wuRunEnd = wuRunStart; wuRunEnd < wuLen; wuRunEnd++ {
				// Pre-condition:
				// wuExe[wuRunEnd] not done or locked, and wuExe[wuRunEnd+1] not
				// locked (if it exists)
				if wuRunEnd == wuLen-1 {
					// Last column, end of run
					break
				}
				// Not last column, wuRunEnd+1 exists and is not locked
				if wuExe[wuRunEnd+1].done { // Okay to read done flag, since wuRunEnd+1 is not locked
					// Right column is done, end of run
					break
				}
				// wuExe[wuRunEnd+1] not done   (1)
				if wuRunEnd < wuLen-2 && wuExe[wuRunEnd+2].locked {
					// 2 Columns to right is locked, end of run
					break
				}
				// wuExe[wuRunEnd+2] not locked, if it exists    (2)
				actionCount += len(wuExe[wuRunEnd].Actions)
				if actionCount > 600 {
					// Enough work for 1 worker, end of run
					break
				}
				// Post-condition:
				// wuExe[wuRunEnd+1] not done by (1), and not locked by pre-condition
				// wuExe[wuRunEnd+2] not locked, if it exists, by (2)
				// Increment wuRunEnd and pre-condition holds.
			}
			w.ThinkStats.Actions += actionCount
			// wuExe[i] is not done for wuRunStart <= i <= wuRunEnd
			// wuExe[i] is not locked for wuRunStart-1 <= i <= wuRunEnd+1, 0 <= i < wuLen
			// Lock this run of workUnits and launch a worker
			if wuRunStart > 0 {
				wuExe[wuRunStart-1].locked = true
			}
			for j := wuRunStart; j <= wuRunEnd; j++ {
				wuExe[j].locked = true
			}
			if wuRunEnd < wuLen-1 {
				wuExe[wuRunEnd+1].locked = true
			}
			// Allocate an AA for the worker
			aa := AllocateAA(w.ticks + 1)
			workerAAs = append(workerAAs, aa)
			// Launch worker
			wgWorkers.Add(1)
			//fmt.Println("worker start", wuRunStart, wuRunEnd, w.ticks)
			go worker(wuRunStart, wuRunEnd, aa)
			w.ThinkStats.Workers++
		} else { // wuRunStart == -1
			// no run found, process any closed ActionAccumulators and wait
			for i := range workerAAs {
				if workerAAs[i] != nil && workerAAs[i].IsClosed() {
					w.process(workerAAs[i], true) // cannot process entities until end-of-tick
				}
			}
			if moreWork {
				// wait for the existing workers to finish, then find new runs
				wgWorkers.Wait()
			}
		}
	}
	// moreWork == false
	wgWorkers.Wait()
	// end-of-tick
	// All workunits are done, process remaining ActionAccumulators
	for i := range workerAAs {
		if workerAAs[i] != nil {
			if !workerAAs[i].IsClosed() {
				panic("unclosed AA")
			}
			w.process(workerAAs[i], false)
			ReleaseAA(workerAAs[i])
			workerAAs[i] = nil
		}
	}
	// clear WU_EXECUTE for later reuse as WU_BUFFER
	for k := range w.workUnits[WU_EXECUTE] {
		v := &w.workUnits[WU_EXECUTE][k]
		v.Actions = v.Actions[:0]
		v.done = false
		v.locked = false
	}
	w.ThinkStats.Elapsed += time.Since(start)
}
