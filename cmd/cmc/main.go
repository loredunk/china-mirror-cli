package main

import (
	"fmt"
	"os"

	"github.com/loredunk/china-mirror/cmd/cmc/cli"
	"github.com/loredunk/china-mirror/internal/sysexits"

	// Register built-in adapters by importing them for their init() side effect.
	_ "github.com/loredunk/china-mirror/internal/adapter/node"
	_ "github.com/loredunk/china-mirror/internal/adapter/python"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(sysexits.Software)
	}
}
