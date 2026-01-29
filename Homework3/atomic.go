package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// 1. atomic
func atomicfunc() {
	var ops_race uint64
	var ops atomic.Uint64

	var wg sync.WaitGroup

	for range 50 {
		wg.Go(func() {
			for range 1000 {

				ops.Add(1)
				ops_race++
			}
		})
	}

	wg.Wait()

	fmt.Println("ops:", ops.Load())
	fmt.Println("race ops:", ops_race)
}
