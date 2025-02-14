package mapreduce

import (
	"fmt"
	"sync"
)

//
// schedule() starts and waits for all tasks in the given phase (Map
// or Reduce). the mapFiles argument holds the names of the files that
// are the inputs to the map phase, one per map task. nReduce is the
// number of reduce tasks. the registerChan argument yields a stream
// of registered workers; each item is the worker's RPC address,
// suitable for passing to call(). registerChan will yield all
// existing registered workers (if any) and new ones as they register.
//
func schedule(jobName string, mapFiles []string, nReduce int, phase jobPhase, registerChan chan string) {

	var ntasks int
	var nOther int // number of inputs (for reduce) or outputs (for map)
	switch phase {
	case mapPhase:
		ntasks = len(mapFiles)
		nOther = nReduce
	case reducePhase:
		ntasks = nReduce
		nOther = len(mapFiles)
	}

	var wg sync.WaitGroup
	for idx := 0; idx < ntasks; idx++ {
		wg.Add(1)
		go func(TaskNumber int) {
			defer wg.Done()
			for {
				workerAddr := <-registerChan

				var arg DoTaskArgs
				arg.JobName = jobName
				arg.NumOtherPhase = nOther
				arg.Phase = phase
				arg.TaskNumber = TaskNumber
				if phase == mapPhase {
					arg.File = mapFiles[TaskNumber]
				} else {
					arg.File = ""
				}

				ret := call(workerAddr, "Worker.DoTask", arg, nil)

				if ret == true {
					go func() { registerChan <- workerAddr }()
					break
				}
			}
		}(idx)
	}
	fmt.Printf("Schedule: %v %v tasks (%d I/Os)\n", ntasks, phase, nOther)

	wg.Wait()
	// All ntasks tasks have to be scheduled on workers, and only once all of
	// them have been completed successfully should the function return.
	// Remember that workers may fail, and that any given worker may finish
	// multiple tasks.
	//
	//
	//

	fmt.Printf("Schedule: %v phase done\n", phase)
}
