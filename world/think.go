package world

import (
	"runtime"
	"sync"
)

func (w *World) Think() {
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
	// Worker sub-function
	wgWorkers := sync.WaitGroup{}
	worker := func(wuRunStart, wuRunEnd int, aa *ActionAccumulator) {
		for i := wuRunStart; i <= wuRunEnd; i++ {
			for _, action := range wuExe[i].Actions {
				action(aa)
			}
			wuExe[i].done = true
		}
		// assigned workUnits are done, now unlock
		if wuRunStart > 0 {
			wuExe[wuRunStart-1].locked = false
		}
		for i := wuRunStart; i < wuRunEnd; i++ {
			wuExe[i].locked = false
		}
		if wuRunEnd < wuLen-1 {
			wuExe[wuRunEnd+1].locked = false
		}
		wgWorkers.Done()
	}
	// Collect the ActionAccumulators given to the workers here
	workerAAs := make([]*ActionAccumulator, 0)
	// Divide the workUnits into groups of a reasonable size, and launch workers
	// to process the workUnits
	moreWork := true
	for moreWork { // Loop until all workUnits are done
		moreWork = false
		// TODO remove
		//for i, v := range wuExe {
		//fmt.Println("wuExe", w.ticks, i, v.X, v.done, v.locked)
		//}
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
				if actionCount > 10000 {
					// Enough work for 1 worker, end of run
					break
				}
			}
			// wuRunEnd <= wuLen, and all work units between wuRenStart and
			// wuRunEnd inclusive can and will now be processed by a new worker.
			// Lock workUnits, worker responsible for unlocking
			if wuRunStart > 0 {
				wuExe[wuRunStart-1].locked = true
			}
			for j := wuRunStart; j < wuRunEnd; j++ {
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
			//fmt.Println("tick", w.Now(), "launcher worker", wuRunStart, wuRunEnd, actionCount)
			go worker(wuRunStart, wuRunEnd, aa)
		} else {
			// no run found
			if moreWork {
				// give workers a chance to work before looking for another run
				runtime.Gosched()
				wgWorkers.Wait()
			}
		}
	}
	// moreWork == false
	wgWorkers.Wait()
	// All workunits are done, process ActionAccumulators
	for _, aa := range workerAAs {
		w.process(aa)
	}
	// clear WU_EXECUTE for later reuse as WU_BUFFER
	for k := range w.workUnits[WU_EXECUTE] {
		v := &w.workUnits[WU_EXECUTE][k]
		v.Actions = v.Actions[:0]
		v.done = false
		v.locked = false
	}
}

/*
	type workerCommand struct {
		WU                  []workUnit
		AA                  *ActionAccumulator
		lockStart, lockStop int
	}
	type workerResponse struct {
		Commands chan<- workerCommand
		AA       *ActionAccumulator
	}
	wg := sync.WaitGroup{}
	responseChannel := make(chan workerResponse, 2)
	worker := func() {
		defer wg.Done()
		cc := make(chan workerCommand, 2)
		responseChannel <- workerResponse{
			Commands: cc,
		}
		for c := range cc {
			for _, wu := range c.WU {
				for _, action := range wu.Actions {
					action(c.AA)
				}
			}
			for i := c.lockStart; i < c.lockStop; i++ {
				wuExe[i].locked = false
			}
			responseChannel <- workerResponse{
				Commands: cc,
				AA:       c.AA,
			}
		}
	}
	processing := true
processLoop:
	for processing {
		processing = false
		// Read a worker response and find new work for it to do
		resp := <-responseChannel
		// Iterate over workUnits to execute this Tick
		for k := range wuExe {
			if wuExe[k].done == true {
				// This workUnit has already been executed
				continue
			}
			if len(wuExe[k].Actions) == 0 {
				// This workUnit is empty, short circuit it
				wuExe[k].done = true
				continue
			}
			// if this is reached, there are still more workUnits to process
			processing = true
			if wuExe[k].locked ||
				(k > 0 && wuExe[k-1].locked) ||
				(k < wuLen-1 && wuExe[k+1].locked) {
				// workUnit for column k or a neighbor is locked, can't process
				continue
			}
			// column 'k' can be processed. lock it and its neighbors
			actionCount := len(wuExe[k].Actions)
			exeStart := k
			lockStart := k
			lockStop := k
			if k > 0 {
				lockStart = k - 1
			}
			// grab more columns to the right if possible
			for k < wuLen-2 && wuExe[k+1].done == false && wuExe[k+2].locked == false && actionCount < 2000 {
				k++
				actionCount += len(wuExe[k].Actions)
			}
			exeStop := k + 1
			if exeStop == wuLen-1 && wuExe[wuLen-1].done == false {
				actionCount += len(wuExe[wuLen-1].Actions)
				exeStop = wuLen
			}
			lockStop = exeStop + 1
			if lockStop > wuLen {
				lockStop = wuLen
			}
			for i := lockStart; i < lockStop; i++ {
				wuExe[i].locked = true
			}
			for i := exeStart; i < exeStop; i++ {
				wuExe[i].done = true
			}
			// Send workUnit to worker
			//fmt.Println(lockStart, exeStart, exeStop, lockStop)
			resp.Commands <- workerCommand{
				WU:        wuExe[exeStart:exeStop],
				AA:        AllocateAA(w.ticks + 1),
				lockStart: lockStart,
				lockStop:  lockStop,
			}
			w.ActionCount += actionCount
			// Start a new worker, in the hope there's more work to be done
			wg.Add(1)
			go worker()
			w.process(resp.AA)
			ReleaseAA(resp.AA)
			continue processLoop
		}
		w.process(resp.AA)
		ReleaseAA(resp.AA)
		// no work for worker, shut it down
		close(resp.Commands)
	}
	// shut down remaining workers
	for resp := range responseChannel {
		w.process(resp.AA)
		ReleaseAA(resp.AA)
		close(resp.Commands)
	}
	// clear WU_EXECUTE for later reuse as WU_BUFFER
	for k := range w.workUnits[WU_EXECUTE] {
		v := &w.workUnits[WU_EXECUTE][k]
		v.Actions = v.Actions[:0]
		v.done = false
		v.locked = false
	}
}
*/
