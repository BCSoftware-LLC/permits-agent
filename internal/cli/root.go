package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.1.0"

func Execute() error {
	rootCmd := &cobra.Command{
		Use:               "abc-agent",
		Short:             "Read-only California ABC licensing data CLI",
		SilenceUsage:      true,
		SilenceErrors:     false,
		Version:           Version,
		PersistentPreRunE: validateFlags,
	}
	rootCmd.SetVersionTemplate("abc-agent {{ .Version }}\n")
	rootCmd.AddCommand(newRefreshCmd())
	rootCmd.AddCommand(newStatsCmd())
	rootCmd.AddCommand(newStatusesCmd())
	rootCmd.AddCommand(newAddressCmd())
	rootCmd.AddCommand(newGetCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newPendingCmd())
	rootCmd.AddCommand(newOverdueCmd())
	rootCmd.AddCommand(newMcpCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newReferenceCmds()...)
	rootCmd.AddCommand(newAreaCmd())
	rootCmd.AddCommand(newExpiringCmd())
	rootCmd.AddCommand(newHistoryCmds()...)
	rootCmd.AddCommand(newDigestCmd())
	rootCmd.AddCommand(newVersionCliCmd())
	return rootCmd.Execute()
}

func newVersionCliCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  exact(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("abc-agent %s\n", Version)
		},
	}
}
