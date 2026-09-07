package cli

import (
	"github.com/BCSoftware-LLC/permits-agent/internal/mcpserver"
	"github.com/spf13/cobra"
)

func newMcpCmd() *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error { return mcpserver.Serve() }
	mcp := &cobra.Command{Use: "mcp", Short: "Run the MCP stdio server", Example: "  abc-agent mcp", Args: exact(0), RunE: run}
	serve := &cobra.Command{Use: "serve", Short: "Alias for mcp", Example: "  abc-agent serve", Args: exact(0), RunE: run}
	mcp.AddCommand(serve)
	return mcp
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{Use: "serve", Short: "Run the MCP stdio server", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error { return mcpserver.Serve() }}
}
