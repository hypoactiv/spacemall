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
	w.process(taTmp)
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
	for moreWork { // Loop until all workUnits are done
		moreWork = false
		// Find the start of a run of workUnits
		wuRunStart := -1
		wuRunEnd := 0
		for i, wu := range wuExe {
			if wu.done {
				continue
			}
			// at least one workUnit is not done
			moreWork = true
			if wu.locked {
				continue
			}
			// Can this workUnit be processed now?
			if i > 0 && wuExe[i-1].locked == true {
				// No, workUnit for left neighboring column is locked
				continue
			}
			if i < wuLen-1 && wuExe[i+1].locked == true {
				// No, workUnit for right neighboring column is locked
				continue
			}
			// This workUnit can be processed, start a new run
			wuRunStart = i
			break
		}
		if wuRunStart != -1 {
			actionCount := len(wuExe[wuRunStart].Actions)
			// Found the start of a run of workUnits, now find its end
			for wuRunEnd = wuRunStart; wuRunEnd < wuLen; wuRunEnd++ {
				if wuRunEnd == wuLen-1 {
					// Last column, end of run
					break
				}
				// Not last column, check right column
				if wuExe[wuRunEnd+1].done || wuExe[wuRunEnd+1].locked {
					// Right column is done or locked, end of run
					break
				}
				actionCount += len(wuExe[wuRunEnd].Actions)
				if actionCount > 300 {
					// Enough work for 1 worker, end of run
					break
				}
			}
			w.ThinkStats.Actions += actionCount
			// wuRunEnd < wuLen, and all work units between wuRenStart and
			// wuRunEnd inclusive can and will now be processed by a new worker.
			// Lock workUnits, worker responsible for unlocking
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
			go worker(wuRunStart, wuRunEnd, aa)
			w.ThinkStats.Workers++
		} else { // wuRunStart == -1
			// no run found, process any closed ActionAccumulators and wait
			for i := range workerAAs {
				if workerAAs[i] != nil && workerAAs[i].IsClosed() {
					w.process(workerAAs[i])
					ReleaseAA(workerAAs[i])
					workerAAs[i] = nil
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
	// All workunits are done, process remaining ActionAccumulators
	for i := range workerAAs {
		if workerAAs[i] != nil {
			if !workerAAs[i].IsClosed() {
				panic("unclosed AA")
			}
			w.process(workerAAs[i])
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
