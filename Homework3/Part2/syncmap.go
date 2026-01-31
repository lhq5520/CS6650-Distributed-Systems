package main

import (
	"fmt"
	"sync"
	"time"
)

func Syncmapfunc() {
	var m sync.Map
	// inside each goroutine:

	// after all done, count entries with m.Range(...)
	var wg sync.WaitGroup

	start := time.Now()

	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()

			for i := 0; i < 1000; i++ {
				m.Store(g*1000+i, i)
			}

		}(g)
	}

	wg.Wait()
	elapsed := time.Since(start)

	count := 0
	m.Range(func(key, value any) bool {
		count++
		return true
	})

	fmt.Println(count)
	fmt.Println("Time taken:", elapsed)
}
