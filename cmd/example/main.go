package main

import (
	"fmt"
	"os"

	"github.com/thomas-elliott/go-graph/graph"
)

func main() {
	g := graph.New()
	if err := loadSampleData(g); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("graph has %d nodes\n", g.Len())
	fmt.Printf("neighbours of Belgium: %v\n", g.Neighbours("Belgium", graph.Out, ""))
	fmt.Printf("neighbours of chunk:Bruges#0: %v\n", g.Neighbours("chunk:Bruges#0", graph.Out, ""))
}
