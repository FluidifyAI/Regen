package commands

import (
	"log/slog"

	"github.com/FluidifyAI/Regen/backend/internal/config"
	"github.com/FluidifyAI/Regen/backend/internal/database"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations and exit",
		RunE:  runMigrate,
	}
}

func runMigrate(_ *cobra.Command, _ []string) error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	setupLogging(cfg.LogLevel)

	dbLogLevel := "info"
	if cfg.Environment == "production" {
		dbLogLevel = "warn"
	}
	dbConfig := database.Config{
		URL:          cfg.DatabaseURL,
		MaxOpenConns: cfg.DBMaxOpenConns,
		MaxIdleConns: cfg.DBMaxIdleConns,
		ConnMaxLife:  cfg.DBConnMaxLife,
		LogLevel:     dbLogLevel,
	}
	if err := database.Connect(dbConfig); err != nil {
		return err
	}
	defer database.Close()

	slog.Info("running database migrations...")
	if err := database.RunMigrations(database.DB, "./migrations"); err != nil {
		return err
	}
	slog.Info("migrations complete")
	return nil
}
