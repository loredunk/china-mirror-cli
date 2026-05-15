package main

import (
	"fmt"
	"os"

	"github.com/loredunk/china-mirror/cmd/cmc/cli"
	"github.com/loredunk/china-mirror/internal/sysexits"

	// Register built-in adapters by importing them for their init() side effect.
	_ "github.com/loredunk/china-mirror/internal/adapter/apt"
	_ "github.com/loredunk/china-mirror/internal/adapter/conda"
	_ "github.com/loredunk/china-mirror/internal/adapter/docker"
	_ "github.com/loredunk/china-mirror/internal/adapter/flutter"
	_ "github.com/loredunk/china-mirror/internal/adapter/github"
	_ "github.com/loredunk/china-mirror/internal/adapter/golang"
	_ "github.com/loredunk/china-mirror/internal/adapter/homebrew"
	_ "github.com/loredunk/china-mirror/internal/adapter/node"
	_ "github.com/loredunk/china-mirror/internal/adapter/python"
	_ "github.com/loredunk/china-mirror/internal/adapter/rust"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(sysexits.Software)
	}
}
