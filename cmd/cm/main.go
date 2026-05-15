package main

import (
	"fmt"
	"os"

	"github.com/loredunk/china-mirror/internal/sysexits"
	"github.com/loredunk/china-mirror/cmd/cm/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(sysexits.Software)
	}
}
