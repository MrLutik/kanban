package main

import (
	"os"

	"github.com/kiracore/kanban/cmd/kanban/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
