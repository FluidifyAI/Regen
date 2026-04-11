package main

import (
	"fmt"
	"os"

	"github.com/FluidifyAI/Regen/backend/cmd/regen/commands"
)

func main() {
	if err := commands.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
