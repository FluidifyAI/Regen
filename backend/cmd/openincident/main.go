package main

import (
	"fmt"
	"os"

	"github.com/openincident/openincident/cmd/openincident/commands"
)

func main() {
	if err := commands.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
