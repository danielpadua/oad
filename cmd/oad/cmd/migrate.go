package cmd

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/spf13/cobra"

	"github.com/danielpadua/oad/internal/config"
	"github.com/danielpadua/oad/migrations"
)

var flagMigrateDatabase string

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Manage database schema migrations",
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newMigrator()
		if err != nil {
			return err
		}
		defer m.Close() //nolint:errcheck // close errors do not affect the user-facing migration outcome

		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("applying migrations: %w", err)
		}

		fmt.Println("migrations applied successfully")
		return nil
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back the last migration",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newMigrator()
		if err != nil {
			return err
		}
		defer m.Close() //nolint:errcheck // close errors do not affect the user-facing migration outcome

		if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("rolling back migration: %w", err)
		}

		fmt.Println("migration rolled back")
		return nil
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current migration version",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newMigrator()
		if err != nil {
			return err
		}
		defer m.Close() //nolint:errcheck // close errors do not affect the user-facing migration outcome

		version, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("version: none (no migrations applied)")
			return nil
		}
		if err != nil {
			return fmt.Errorf("querying migration version: %w", err)
		}

		if dirty {
			fmt.Printf("version: %d (dirty — previous migration failed)\n", version)
		} else {
			fmt.Printf("version: %d\n", version)
		}
		return nil
	},
}

func init() {
	for _, sub := range []*cobra.Command{migrateUpCmd, migrateDownCmd, migrateStatusCmd} {
		sub.Flags().StringVar(&flagMigrateDatabase, "database", "", "PostgreSQL DSN (overrides config file and OAD_DATABASE)")
		migrateCmd.AddCommand(sub)
	}
	rootCmd.AddCommand(migrateCmd)
}

// newMigrator resolves the database URL from flags / env / YAML file and
// returns a ready-to-use golang-migrate instance.
func newMigrator() (*migrate.Migrate, error) {
	cfg, err := config.Load(config.CLIOptions{
		ConfigFile: CfgFile,
		Database:   flagMigrateDatabase,
		AuthMode:   "none", // skip provider validation for migrate commands
	})
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("initializing migrator: %w", err)
	}

	return m, nil
}
