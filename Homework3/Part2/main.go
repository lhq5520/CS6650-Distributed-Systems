package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n=================================")
		fmt.Println("Select a function to execute:")
		fmt.Println("1. Atomic Operations Demo")
		fmt.Println("2. Collections Concurrency Demo")
		fmt.Println("3. Mutex Demo")
		fmt.Println("4. RWMutex Demo")
		fmt.Println("5. Sync.Map Demo")
		fmt.Println("6. Buffered File Access Demo")
		fmt.Println("7. Unbuffered File Access Demo")
		fmt.Println("8. Context Switch (Single Thread) Demo")
		fmt.Println("9. Context Switch (All CPUs) Demo")
		fmt.Println("q. Exit")
		fmt.Println("=================================")
		fmt.Print("Enter option (1-9 or q): ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			fmt.Println("\n--- Running Atomic Demo ---")
			atomicfunc()
		case "2":
			fmt.Println("\n--- Running Collections Demo ---")
			collectionsfunc()
		case "3":
			fmt.Println("\n--- Running Mutex Demo ---")
			Mutexfunc()
		case "4":
			fmt.Println("\n--- Running RWMutex Demo ---")
			RWMutex()
		case "5":
			fmt.Println("\n--- Running Sync.Map Demo ---")
			Syncmapfunc()
		case "6":
			fmt.Println("\n--- Running Buffered File Access Demo ---")
			buffedfile()
		case "7":
			fmt.Println("\n--- Running Unbuffered File Access Demo ---")
			unbufferedfile()
		case "8":
			fmt.Println("\n--- Running Context Switch (Single Thread) Demo ---")
			contextSwitchSingle()
		case "9":
			fmt.Println("\n--- Running Context Switch (All CPUs) Demo ---")
			contextSwitchAll()
		case "q", "Q":
			fmt.Println("Exiting program, goodbye!")
			return
		default:
			fmt.Println("⚠️  Invalid option, please enter 1-9 or q")
		}
	}
}
