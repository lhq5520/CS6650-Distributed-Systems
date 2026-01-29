package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

// 6.File Access
func buffedfile() {
	f, err := os.Create("filename.txt")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	start := time.Now()
	for i := 0; i < 100000; i++ {
		w.WriteString(fmt.Sprintf("Line %d\n", i))
	}
	w.Flush()
	elapsed := time.Since(start)

	fmt.Printf("Total time: %s\n", elapsed)

}

func unbufferedfile() {
	f, err := os.Create("filename.txt")
	bufio.NewWriter(f)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	start := time.Now()
	for i := 0; i < 100000; i++ {
		f.Write([]byte(fmt.Sprintf("Line %d\n", i)))
	}
	elapsed := time.Since(start)

	fmt.Printf("Total time: %s\n", elapsed)
}
