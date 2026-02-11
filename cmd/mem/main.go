package main

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/fang"
)

// version is set via ldflags at build time
var version = "dev"

func main() {
	ctx := context.Background()

	if tryExternalCommand(ctx) {
		return
	}

	rootCmd := NewRootCmd(version)
	if err := fang.Execute(ctx, rootCmd); err != nil {
		os.Exit(1)
	}
}

func tryExternalCommand(ctx context.Context) bool {
	if len(os.Args) < 2 {
		return false
	}

	cmd := os.Args[1]
	if cmd == "" || cmd[0] == '-' {
		return false
	}

	if _, err := findExternal(cmd); err != nil {
		return false
	}

	if err := executeExternal(ctx, cmd, os.Args[2:], version); err != nil {
		fmt.Fprintf(os.Stderr, "mem %s: %v\n", cmd, err)
		os.Exit(1)
	}

	return true
}
