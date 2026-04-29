package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Populated at link time via -ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version, commit, and build date",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("oad %s\ncommit:  %s\nbuilt:   %s\n", version, commit, buildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
