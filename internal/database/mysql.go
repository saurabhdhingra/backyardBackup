package database

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// MySQLConnector implements the Connector interface for MySQL databases
type MySQLConnector struct {
	host     string
	port     int
	user     string
	password string
	dbname   string
}

// Connect establishes a connection to the MySQL database
func (c *MySQLConnector) Connect(ctx context.Context, config ConnectConfig) error {
	c.host = config.Host
	c.port = config.Port
	c.user = config.User
	c.password = config.Password
	c.dbname = config.Database

	// Test connection
	args := []string{
		"-h", c.host,
		"-P", fmt.Sprintf("%d", c.port),
		"-u", c.user,
		"-p" + c.password,
		"-e", "SELECT 1",
		c.dbname,
	}

	cmd := exec.CommandContext(ctx, "mysql", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	return nil
}

// Close terminates the database connection
func (c *MySQLConnector) Close() error {
	// No persistent connection to close in this implementation
	return nil
}

// Backup dumps the database to a writer
func (c *MySQLConnector) Backup(ctx context.Context, w io.Writer, tables []string) error {
	args := []string{
		"-h", c.host,
		"-P", fmt.Sprintf("%d", c.port),
		"-u", c.user,
		"-p" + c.password,
		"--single-transaction",
		"--routines",
		"--triggers",
		"--events",
		"--add-drop-database",
		"--databases", c.dbname,
	}

	if len(tables) > 0 {
		args = append(args, "--tables")
		args = append(args, tables...)
	}

	cmd := exec.CommandContext(ctx, "mysqldump", args...)
	cmd.Stdout = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}

	return nil
}

// Restore restores the database from a reader
func (c *MySQLConnector) Restore(ctx context.Context, r io.Reader) error {
	args := []string{
		"-h", c.host,
		"-P", fmt.Sprintf("%d", c.port),
		"-u", c.user,
		"-p" + c.password,
		c.dbname,
	}

	cmd := exec.CommandContext(ctx, "mysql", args...)
	cmd.Stdin = r

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql restore failed: %w", err)
	}

	return nil
}

// ListTables returns a list of all tables in the database
func (c *MySQLConnector) ListTables(ctx context.Context) ([]string, error) {
	args := []string{
		"-h", c.host,
		"-P", fmt.Sprintf("%d", c.port),
		"-u", c.user,
		"-p" + c.password,
		"-N", // Skip column names
		"-B", // Batch mode (tab-separated)
		c.dbname,
		"-e", "SHOW TABLES",
	}

	cmd := exec.CommandContext(ctx, "mysql", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	tables := strings.Split(strings.TrimSpace(string(output)), "\n")
	return tables, nil
}

// GetInfo returns information about the database
func (c *MySQLConnector) GetInfo(ctx context.Context) (*DatabaseInfo, error) {
	args := []string{
		"-h", c.host,
		"-P", fmt.Sprintf("%d", c.port),
		"-u", c.user,
		"-p" + c.password,
		"-N", // Skip column names
		"-B", // Batch mode (tab-separated)
		c.dbname,
		"-e", "SELECT ROUND(SUM(data_length + index_length) / 1024 / 1024, 2) as size, version() as version FROM information_schema.tables WHERE table_schema = DATABASE() GROUP BY version",
	}

	cmd := exec.CommandContext(ctx, "mysql", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get database info: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected output format from database")
	}

	return &DatabaseInfo{
		Size:    fmt.Sprintf("%s MB", parts[0]),
		Version: parts[1],
		Type:    MySQL,
	}, nil
}

// Type returns the database type
func (c *MySQLConnector) Type() DBType {
	return MySQL
} 