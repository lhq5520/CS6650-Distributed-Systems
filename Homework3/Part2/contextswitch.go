package main

import (
	"fmt"
	"runtime"
	"time"
)

// 7.Context Switching
func contextSwitchSingle() {
	runtime.GOMAXPROCS(1)

	ch := make(chan int)

	start := time.Now()

	go func() {
		for i := 0; i < 1000000; i++ {
			ch <- 99
		}
	}()

	for i := 0; i < 1000000; i++ {
		<-ch
	}

	elapsed := time.Since(start)
	avgSwitch := elapsed / (2 * 1000000)

	fmt.Printf("Single thread avg switch: %v\n", avgSwitch)

}

func contextSwitchAll() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	ch := make(chan int)

	start := time.Now()

	go func() {
		for i := 0; i < 1000000; i++ {
			ch <- 99
		}
	}()

	for i := 0; i < 1000000; i++ {
		<-ch
	}

	elapsed := time.Since(start)
	avgSwitch := elapsed / (2 * 1000000)

	fmt.Printf("Multi thread avg switch: %v\n", avgSwitch)

}
