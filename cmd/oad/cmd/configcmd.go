package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/danielpadua/oad/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration utilities",
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the configuration file without starting the server",
	Long: `Loads and validates the configuration from the YAML file, environment variables,
and any supplied flags. Exits with code 0 on success, 1 on any validation error.
No database connection is opened.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.Load(config.CLIOptions{
			ConfigFile:      CfgFile,
			Database:        flagValidateDatabase,
			Addr:            flagValidateAddr,
			AuthMode:        flagValidateAuthMode,
			ShutdownTimeout: flagValidateShutdownTimeout,
			LogLevel:        flagValidateLogLevel,
			LogFormat:       flagValidateLogFormat,
		})
		if err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		fmt.Println("configuration is valid")
		return nil
	},
}

var (
	flagValidateDatabase        string
	flagValidateAddr            string
	flagValidateAuthMode        string
	flagValidateShutdownTimeout string
	flagValidateLogLevel        string
	flagValidateLogFormat       string
)

func init() {
	configValidateCmd.Flags().StringVar(&flagValidateDatabase, "database", "", "PostgreSQL DSN")
	configValidateCmd.Flags().StringVar(&flagValidateAddr, "addr", "", "bind address [host]:port")
	configValidateCmd.Flags().StringVar(&flagValidateAuthMode, "auth-mode", "", "jwt | mtls | both | none")
	configValidateCmd.Flags().StringVar(&flagValidateShutdownTimeout, "shutdown-timeout", "", "graceful shutdown deadline, e.g. 30s")
	configValidateCmd.Flags().StringVar(&flagValidateLogLevel, "log-level", "", "debug | info | warn | error")
	configValidateCmd.Flags().StringVar(&flagValidateLogFormat, "log-format", "", "json | text")

	configCmd.AddCommand(configValidateCmd)
	rootCmd.AddCommand(configCmd)
}
