// Command toy is a fixture whose output can be mutated to produce each class
// of documentation failure on demand.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: toy <status|balance>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "status":
		fmt.Println(`{"state": "SUCCEEDED", "height": "12"}`)
	case "balance":
		fmt.Println(`{"balance": "10", "symbol": "DEMO"}`)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(1)
	}
}
