package database

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// MongoDBConnector implements the Connector interface for MongoDB databases
type MongoDBConnector struct {
	host     string
	port     int
	user     string
	password string
	dbname   string
	uri      string
}

// Connect establishes a connection to the MongoDB database
func (c *MongoDBConnector) Connect(ctx context.Context, config ConnectConfig) error {
	c.host = config.Host
	c.port = config.Port
	c.user = config.User
	c.password = config.Password
	c.dbname = config.Database

	// Build MongoDB URI
	if c.user != "" && c.password != "" {
		c.uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", c.user, c.password, c.host, c.port, c.dbname)
	} else {
		c.uri = fmt.Sprintf("mongodb://%s:%d/%s", c.host, c.port, c.dbname)
	}

	// Test connection
	args := []string{
		"--uri", c.uri,
		"--eval", "db.runCommand({ping: 1})",
	}

	cmd := exec.CommandContext(ctx, "mongosh", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	return nil
}

// Close terminates the database connection
func (c *MongoDBConnector) Close() error {
	// No persistent connection to close in this implementation
	return nil
}

// Backup dumps the database to a writer
func (c *MongoDBConnector) Backup(ctx context.Context, w io.Writer, collections []string) error {
	args := []string{
		"--uri", c.uri,
		"--archive",
		"--gzip",
	}

	if len(collections) > 0 {
		for _, collection := range collections {
			args = append(args, "--collection", collection)
		}
	}

	cmd := exec.CommandContext(ctx, "mongodump", args...)
	cmd.Stdout = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongodump failed: %w", err)
	}

	return nil
}

// Restore restores the database from a reader
func (c *MongoDBConnector) Restore(ctx context.Context, r io.Reader) error {
	args := []string{
		"--uri", c.uri,
		"--archive",
		"--gzip",
		"--drop", // Drop existing collections before restore
	}

	cmd := exec.CommandContext(ctx, "mongorestore", args...)
	cmd.Stdin = r

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongorestore failed: %w", err)
	}

	return nil
}

// ListTables returns a list of all collections in the database
func (c *MongoDBConnector) ListTables(ctx context.Context) ([]string, error) {
	args := []string{
		"--uri", c.uri,
		"--quiet",
		"--eval", "db.getCollectionNames()",
	}

	cmd := exec.CommandContext(ctx, "mongosh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	// Parse the output which is in the format: [ "collection1", "collection2", ... ]
	collections := strings.TrimSpace(string(output))
	collections = strings.TrimPrefix(collections, "[")
	collections = strings.TrimSuffix(collections, "]")
	collections = strings.ReplaceAll(collections, "\"", "")
	collections = strings.ReplaceAll(collections, " ", "")

	if collections == "" {
		return []string{}, nil
	}

	return strings.Split(collections, ","), nil
}

// GetInfo returns information about the database
func (c *MongoDBConnector) GetInfo(ctx context.Context) (*DatabaseInfo, error) {
	// Get database stats
	args := []string{
		"--uri", c.uri,
		"--quiet",
		"--eval", "JSON.stringify(db.stats())",
	}

	cmd := exec.CommandContext(ctx, "mongosh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get database stats: %w", err)
	}

	// Get MongoDB version
	versionArgs := []string{
		"--uri", c.uri,
		"--quiet",
		"--eval", "db.version()",
	}

	versionCmd := exec.CommandContext(ctx, "mongosh", versionArgs...)
	versionOutput, err := versionCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get MongoDB version: %w", err)
	}

	// Extract size from stats output (dataSize is in bytes)
	stats := strings.TrimSpace(string(output))
	if strings.Contains(stats, "dataSize") {
		sizeMB := float64(0)
		fmt.Sscanf(stats, `{"dataSize":%f`, &sizeMB)
		sizeMB = sizeMB / 1024 / 1024 // Convert to MB
		return &DatabaseInfo{
			Size:    fmt.Sprintf("%.2f MB", sizeMB),
			Version: strings.TrimSpace(string(versionOutput)),
			Type:    MongoDB,
		}, nil
	}

	return nil, fmt.Errorf("failed to parse database stats")
}

// Type returns the database type
func (c *MongoDBConnector) Type() DBType {
	return MongoDB
} 