package database

import (
	"context"
	"fmt"
	"io"
	"os"
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
	if c.host == "" {
		return fmt.Errorf("database connection not initialized")
	}

	// Create pg_restore command
	args := []string{
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-c", // Clean (drop) database objects before recreating
		"-v", // Verbose mode
		"--if-exists", // Don't error if object doesn't exist
	}

	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))
	cmd.Stdin = r
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore failed: %w", err)
	}

	return nil
}

// ListTables returns a list of all tables in the database
func (c *PostgreSQLConnector) ListTables(ctx context.Context) ([]string, error) {
	if c.host == "" {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Create psql command to list tables
	query := `SELECT tablename FROM pg_tables WHERE schemaname = 'public';`
	args := []string{
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-t", // Tuple only output
		"-A", // Unaligned output mode
		"-c", query,
	}

	cmd := exec.CommandContext(ctx, "psql", args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	// Parse output
	tables := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(tables) == 1 && tables[0] == "" {
		return []string{}, nil
	}

	return tables, nil
}

// GetInfo returns information about the database
func (c *PostgreSQLConnector) GetInfo(ctx context.Context) (map[string]string, error) {
	if c.host == "" {
		return nil, fmt.Errorf("database connection not initialized")
	}

	info := make(map[string]string)

	// Get database size
	sizeQuery := `
		SELECT pg_size_pretty(pg_database_size(current_database())) as size,
			   pg_database_size(current_database()) as size_bytes;
	`
	args := []string{
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-t", // Tuple only output
		"-A", // Unaligned output mode
		"-c", sizeQuery,
	}

	cmd := exec.CommandContext(ctx, "psql", args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) >= 2 {
		info["size"] = parts[0]
		info["size_bytes"] = parts[1]
	}

	// Get version information
	versionQuery := `SELECT version();`
	args = []string{
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-t", // Tuple only output
		"-A", // Unaligned output mode
		"-c", versionQuery,
	}

	cmd = exec.CommandContext(ctx, "psql", args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))

	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get version info: %w", err)
	}

	info["version"] = strings.TrimSpace(string(output))

	// Get table count
	tableCountQuery := `SELECT count(*) FROM pg_tables WHERE schemaname = 'public';`
	args = []string{
		"-h", c.host,
		"-p", fmt.Sprintf("%d", c.port),
		"-U", c.user,
		"-d", c.dbname,
		"-t", // Tuple only output
		"-A", // Unaligned output mode
		"-c", tableCountQuery,
	}

	cmd = exec.CommandContext(ctx, "psql", args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", c.password))

	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get table count: %w", err)
	}

	info["table_count"] = strings.TrimSpace(string(output))

	return info, nil
}

// Type returns the database type
func (c *PostgreSQLConnector) Type() DBType {
	return PostgreSQL
} 