package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB creates an in-memory SQLite database for testing
// This provides a fast, isolated database for each test
func NewTestDB() (*gorm.DB, error) {
	// Use SQLite in-memory mode with shared cache
	// This allows multiple connections (including transactions) to see the same database
	// Mode=memory creates an in-memory database
	// cache=shared allows all connections to share the same database instance
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Silent mode for cleaner test output
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}
