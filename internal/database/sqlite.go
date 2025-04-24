package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteConnector implements the Connector interface for SQLite databases
type SQLiteConnector struct {
	db       *sql.DB
	filePath string
}

// Connect establishes a connection to the SQLite database
func (c *SQLiteConnector) Connect(ctx context.Context, config ConnectConfig) error {
	if config.Type != SQLite {
		return fmt.Errorf("invalid database type: %s, expected: %s", config.Type, SQLite)
	}

	if config.FilePath == "" {
		return fmt.Errorf("file path is required for SQLite database")
	}

	c.filePath = config.FilePath

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Connect to database
	db, err := sql.Open("sqlite3", c.filePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection options
	db.SetMaxOpenConns(1) // SQLite supports only one writer at a time
	
	// Check connection with context
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	c.db = db
	return nil
}

// Close terminates the database connection
func (c *SQLiteConnector) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Backup dumps the database to a writer
func (c *SQLiteConnector) Backup(ctx context.Context, w io.Writer, tables []string) error {
	if c.db == nil {
		return fmt.Errorf("database connection not established")
	}

	// For SQLite, we can simply copy the database file
	file, err := os.Open(c.filePath)
	if err != nil {
		return fmt.Errorf("failed to open database file: %w", err)
	}
	defer file.Close()

	// Copy the database file to the writer
	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("failed to copy database: %w", err)
	}

	return nil
}

// Restore restores the database from a reader
func (c *SQLiteConnector) Restore(ctx context.Context, r io.Reader) error {
	if c.db == nil {
		return fmt.Errorf("database connection not established")
	}

	// Close the current connection
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	c.db = nil

	// Create a temporary file
	tempFile := c.filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Copy the reader to the temporary file
	if _, err := io.Copy(file, r); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to write database: %w", err)
	}
	file.Close()

	// Replace the original file with the temporary file
	if err := os.Rename(tempFile, c.filePath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to replace database file: %w", err)
	}

	// Reconnect to the database
	db, err := sql.Open("sqlite3", c.filePath)
	if err != nil {
		return fmt.Errorf("failed to reopen database: %w", err)
	}

	// Set connection options
	db.SetMaxOpenConns(1)
	
	// Check connection with context
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping restored database: %w", err)
	}

	c.db = db
	return nil
}

// ListTables returns a list of all tables in the database
func (c *SQLiteConnector) ListTables(ctx context.Context) ([]string, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database connection not established")
	}

	// Query to get all table names
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`
	
	// Execute the query
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	// Collect table names
	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	return tables, nil
}

// GetInfo returns information about the database
func (c *SQLiteConnector) GetInfo(ctx context.Context) (*DatabaseInfo, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database connection not established")
	}

	// Get database file info
	fileInfo, err := os.Stat(c.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get database file info: %w", err)
	}

	// Get list of tables
	tables, err := c.ListTables(ctx)
	if err != nil {
		return nil, err
	}

	// Get table sizes
	tableSizes := make(map[string]int64)
	for _, table := range tables {
		// This is an approximation as SQLite doesn't provide easy table size information
		var count int64
		err := c.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", table)).Scan(&count)
		if err != nil {
			continue
		}
		tableSizes[table] = count // This is row count, not actual size
	}

	// Create database info
	info := &DatabaseInfo{
		Name:        filepath.Base(c.filePath),
		Size:        fileInfo.Size(),
		Tables:      tables,
		TableSizes:  tableSizes,
		LastUpdated: fileInfo.ModTime().Format(time.RFC3339),
	}

	return info, nil
}

// Type returns the database type
func (c *SQLiteConnector) Type() DBType {
	return SQLite
} 