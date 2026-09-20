package main

import (
	"os"

	"github.com/BCSoftware-LLC/permits-agent/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
