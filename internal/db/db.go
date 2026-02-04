package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"guardrails/internal/models"
)

const (
	// GuardrailsDir is the directory name for guardrails data
	GuardrailsDir = ".guardrails"
	// DBFileName is the database filename within the guardrails directory
	DBFileName = "db.sqlite"
	// SchemaVersion is the current schema version
	SchemaVersion = "1"
)

var (
	db   *gorm.DB
	dbMu sync.RWMutex
)

// InitDB initializes the database connection and runs migrations
func InitDB(dbPath string) (*gorm.DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Configure GORM with silent logger for production
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	// Open SQLite database
	database, err := gorm.Open(sqlite.Open(dbPath), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool for SQLite
	// Note: SQLite supports multiple readers but only one writer.
	// Setting a small pool allows concurrent reads within transactions.
	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)

	// Enable WAL mode for better concurrency (multiple readers, single writer)
	// and set busy timeout to wait instead of immediately failing on lock
	if err := database.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	if err := database.Exec("PRAGMA busy_timeout=5000").Error; err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Run migrations
	if err := runMigrations(database); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	dbMu.Lock()
	db = database
	dbMu.Unlock()
	return database, nil
}

// runMigrations runs all database migrations
func runMigrations(database *gorm.DB) error {
	return database.AutoMigrate(
		&models.Task{},
		&models.Dependency{},
		&models.Config{},
		&models.Gate{},
		&models.GateTaskLink{},
		&models.GateRun{},
		&models.Template{},
		&models.TaskHistory{},
	)
}

// GetDB returns the current database connection
func GetDB() *gorm.DB {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return db
}

// SetDB sets the database connection (used for testing)
func SetDB(database *gorm.DB) {
	dbMu.Lock()
	defer dbMu.Unlock()
	db = database
}

// CloseDB closes the database connection
func CloseDB() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	err = sqlDB.Close()
	db = nil
	return err
}

// FindProjectRoot searches for a guardrails project root
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	dir := cwd
	for {
		guardrailsPath := filepath.Join(dir, GuardrailsDir)
		if info, err := os.Stat(guardrailsPath); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a guardrails project (no %s/ found)", GuardrailsDir)
		}
		dir = parent
	}
}

// GetDefaultDBPath returns the default database path for the current project
func GetDefaultDBPath() (string, error) {
	root, err := FindProjectRoot()
	if err != nil {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return "", cwdErr
		}
		return filepath.Join(cwd, GuardrailsDir, DBFileName), nil
	}
	return filepath.Join(root, GuardrailsDir, DBFileName), nil
}

// EnsureInitialized checks if the database is initialized
func EnsureInitialized() error {
	dbMu.RLock()
	isNil := db == nil
	dbMu.RUnlock()

	if isNil {
		dbPath, err := GetDefaultDBPath()
		if err != nil {
			return err
		}
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("guardrails not initialized. Run 'gur init' first")
		}
		_, err = InitDB(dbPath)
		return err
	}
	return nil
}

// SetConfig sets a configuration value
func SetConfig(key, value string) error {
	config := models.Config{Key: key, Value: value}
	return db.Save(&config).Error
}

// GetConfig gets a configuration value
func GetConfig(key string) (string, error) {
	var config models.Config
	err := db.Where("key = ?", key).First(&config).Error
	if err != nil {
		return "", err
	}
	return config.Value, nil
}
