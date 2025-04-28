package database

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// PostgreSQLConnector implements the Connector interface for PostgreSQL databases
type PostgreSQLConnector struct {
	host     string
	port     int
	user     string
	password string
	dbname   string
	sslmode  string
}

// Connect establishes a connection to the PostgreSQL database
func (c *PostgreSQLConnector) Connect(ctx context.Context, config ConnectConfig) error {
	c.host = config.Host
	c.port = config.Port
	c.user = config.User
	c.password = config.Password
	c.dbname = config.Database
	c.sslmode = "disable" // Default to disable, can be made configurable

	// Test connection using psql
	cmd := exec.CommandContext(ctx, "psql",
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-c", "SELECT 1")
	
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	return nil
}

// Close terminates the database connection
func (c *PostgreSQLConnector) Close() error {
	// No persistent connection to close in this implementation
	return nil
}

// Backup dumps the database to a writer
func (c *PostgreSQLConnector) Backup(ctx context.Context, w io.Writer, tables []string) error {
	args := []string{
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-F", "c", // Custom format
		"-b", // Include large objects
		"-v", // Verbose
		"-C", // Include commands to create database
	}

	if len(tables) > 0 {
		args = append(args, "-t")
		args = append(args, strings.Join(tables, " -t "))
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))
	cmd.Stdout = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	return nil
}

// Restore restores the database from a reader
func (c *PostgreSQLConnector) Restore(ctx context.Context, r io.Reader) error {
	cmd := exec.CommandContext(ctx, "pg_restore",
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-v", // Verbose
		"-c", // Clean (drop) database objects before recreating
		"-1", // Process everything in a single transaction
		"-")  // Read from stdin

	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))
	cmd.Stdin = r

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore failed: %w", err)
	}

	return nil
}

// ListTables returns a list of all tables in the database
func (c *PostgreSQLConnector) ListTables(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "psql",
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-t", // Tuple only, no header
		"-A", // Unaligned output
		"-c", "SELECT tablename FROM pg_tables WHERE schemaname = 'public'")

	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	tables := strings.Split(strings.TrimSpace(string(output)), "\n")
	return tables, nil
}

// GetInfo returns information about the database
func (c *PostgreSQLConnector) GetInfo(ctx context.Context) (*DatabaseInfo, error) {
	cmd := exec.CommandContext(ctx, "psql",
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-t", // Tuple only, no header
		"-A", // Unaligned output
		"-c", "SELECT pg_size_pretty(pg_database_size(current_database())), version()")

	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get database info: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected output format from database")
	}

	return &DatabaseInfo{
		Size:    parts[0],
		Version: parts[1],
		Type:    PostgreSQL,
	}, nil
}

// Type returns the database type
func (c *PostgreSQLConnector) Type() DBType {
	return PostgreSQL
} 