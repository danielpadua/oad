package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// CfgFile is the path to the YAML config file, set by the --config flag.
// Exported so sub-packages can read it; populated before any RunE is called.
var CfgFile string

var rootCmd = &cobra.Command{
	Use:          "oad",
	Short:        "Open Authoritative Directory — attribute repository for PDP authorization",
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&CfgFile, "config", "c", "", "path to YAML config file")
}

// Execute is the entrypoint called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
