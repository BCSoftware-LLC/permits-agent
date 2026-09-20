package main

import (
	"fmt"
	"os"

	"github.com/BCSoftware-LLC/permits-agent/internal/cli"
	"github.com/BCSoftware-LLC/permits-agent/internal/mcpserver"
)

func main() {
	mcpserver.Version = cli.Version
	if err := mcpserver.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
