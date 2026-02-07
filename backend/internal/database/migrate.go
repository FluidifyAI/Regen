package database

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/gorm"
)

// createMigrateInstance creates and configures a migrate instance
func createMigrateInstance(db *gorm.DB, migrationsPath string) (*migrate.Migrate, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return m, nil
}

// RunMigrations runs all pending database migrations
func RunMigrations(db *gorm.DB, migrationsPath string) error {
	m, err := createMigrateInstance(db, migrationsPath)
	if err != nil {
		return err
	}

	// Note: We don't close the migrate instance here because we used WithInstance()
	// which means we don't own the database connection lifecycle.
	// The source (file system) will be closed automatically on process exit.

	// Check current migration state
	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d: manual intervention required (migrations cannot proceed)", version)
	}

	if errors.Is(err, migrate.ErrNilVersion) {
		slog.Info("no migrations applied yet")
	} else {
		slog.Info("current migration state",
			"version", version,
			"dirty", dirty,
		)
	}

	// Run migrations
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("no new migrations to apply")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Log final state
	version, _, err = m.Version()
	if err != nil {
		return fmt.Errorf("failed to get migration version after applying: %w", err)
	}

	slog.Info("migrations applied successfully",
		"version", version,
	)

	return nil
}

// RollbackMigration rolls back the last migration
func RollbackMigration(db *gorm.DB, migrationsPath string) error {
	m, err := createMigrateInstance(db, migrationsPath)
	if err != nil {
		return err
	}

	// Note: We don't close the migrate instance here because we used WithInstance()
	// which means we don't own the database connection lifecycle.

	// Check current version
	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return fmt.Errorf("no migrations to rollback")
		}
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d: manual intervention required (cannot rollback)", version)
	}

	slog.Info("rolling back migration",
		"current_version", version,
		"dirty", dirty,
	)

	// Rollback one step
	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	// Log new state
	version, _, err = m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version after rollback: %w", err)
	}

	if errors.Is(err, migrate.ErrNilVersion) {
		slog.Info("rollback complete, no migrations applied")
	} else {
		slog.Info("rollback complete",
			"new_version", version,
		)
	}

	return nil
}
