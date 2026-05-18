package main

import (
	"fmt"
	"os"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/cli"
)

func main() {
	if err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
