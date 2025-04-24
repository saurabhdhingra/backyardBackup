package database

import (
	"context"
	"io"
)

// DBType represents a database type
type DBType string

const (
	// MySQL database type
	MySQL DBType = "mysql"
	// PostgreSQL database type
	PostgreSQL DBType = "postgres"
	// MongoDB database type
	MongoDB DBType = "mongodb"
	// SQLite database type
	SQLite DBType = "sqlite"
)

// ConnectConfig holds configuration for connecting to a database
type ConnectConfig struct {
	Type     DBType
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	FilePath string // For SQLite
	Options  map[string]string
}

// DatabaseInfo contains information about a database
type DatabaseInfo struct {
	Name        string
	Size        int64
	Tables      []string
	TableSizes  map[string]int64
	LastUpdated string
}

// Connector is the interface for database connections and operations
type Connector interface {
	// Connect establishes a connection to the database
	Connect(ctx context.Context, config ConnectConfig) error
	
	// Close terminates the database connection
	Close() error
	
	// Backup dumps the database (or selected tables) to a writer
	Backup(ctx context.Context, w io.Writer, tables []string) error
	
	// Restore restores the database from a reader
	Restore(ctx context.Context, r io.Reader) error
	
	// ListTables returns a list of all tables in the database
	ListTables(ctx context.Context) ([]string, error)
	
	// GetInfo returns information about the database
	GetInfo(ctx context.Context) (*DatabaseInfo, error)
	
	// Type returns the database type
	Type() DBType
} 