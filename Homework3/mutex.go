package main

import (
	"fmt"
	"sync"
	"time"
)

// 3.Mutex
type MapMutex struct {
	mu sync.Mutex
	m  map[int]int
}

func Mutexfunc() {
	sm := MapMutex{}
	sm.m = make(map[int]int)

	var wg sync.WaitGroup

	start := time.Now()

	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()

			for i := 0; i < 1000; i++ {
				sm.mu.Lock()
				sm.m[g*1000+i] = i
				sm.mu.Unlock()
			}

		}(g)
	}

	wg.Wait()
	elapsed := time.Since(start)
	fmt.Println(len(sm.m))
	fmt.Println("Time taken:", elapsed)
}
