package main

import (
	"fmt"
	"os"

	"github.com/lburgazzoli/claude-plugins/tools/k8s-controller-analyzer/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
