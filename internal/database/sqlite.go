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
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Query to get all tables
	query := `
		SELECT name FROM sqlite_master 
		WHERE type='table' 
		AND name NOT LIKE 'sqlite_%'
		ORDER BY name;
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating table rows: %w", err)
	}

	return tables, nil
}

// GetInfo returns information about the database
func (c *SQLiteConnector) GetInfo(ctx context.Context) (map[string]string, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	info := make(map[string]string)

	// Get SQLite version
	var version string
	if err := c.db.QueryRowContext(ctx, "SELECT sqlite_version();").Scan(&version); err != nil {
		return nil, fmt.Errorf("failed to get SQLite version: %w", err)
	}
	info["version"] = version

	// Get database size
	fileInfo, err := os.Stat(c.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get database file info: %w", err)
	}
	info["size"] = fmt.Sprintf("%d", fileInfo.Size())

	// Get table count
	var tableCount int
	query := `
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' 
		AND name NOT LIKE 'sqlite_%';
	`
	if err := c.db.QueryRowContext(ctx, query).Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("failed to get table count: %w", err)
	}
	info["table_count"] = fmt.Sprintf("%d", tableCount)

	// Get total row count across all tables
	tables, err := c.ListTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	var totalRows int64
	for _, table := range tables {
		var rowCount int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %q;", table)
		if err := c.db.QueryRowContext(ctx, query).Scan(&rowCount); err != nil {
			return nil, fmt.Errorf("failed to get row count for table %s: %w", table, err)
		}
		totalRows += rowCount
	}
	info["total_rows"] = fmt.Sprintf("%d", totalRows)

	// Get page size and page count
	var pageSize, pageCount int
	if err := c.db.QueryRowContext(ctx, "PRAGMA page_size;").Scan(&pageSize); err != nil {
		return nil, fmt.Errorf("failed to get page size: %w", err)
	}
	if err := c.db.QueryRowContext(ctx, "PRAGMA page_count;").Scan(&pageCount); err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}
	info["page_size"] = fmt.Sprintf("%d", pageSize)
	info["page_count"] = fmt.Sprintf("%d", pageCount)
	info["allocated_size"] = fmt.Sprintf("%d", int64(pageSize)*int64(pageCount))

	// Get database encoding
	var encoding string
	if err := c.db.QueryRowContext(ctx, "PRAGMA encoding;").Scan(&encoding); err != nil {
		return nil, fmt.Errorf("failed to get database encoding: %w", err)
	}
	info["encoding"] = encoding

	return info, nil
}

// Type returns the database type
func (c *SQLiteConnector) Type() DBType {
	return SQLite
} 