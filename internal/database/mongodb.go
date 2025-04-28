package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	if c.uri == "" {
		return fmt.Errorf("database connection not initialized")
	}

	// Create temporary file to store the backup
	tmpFile, err := os.CreateTemp("", "mongodb-restore-*.archive")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy backup data to temporary file
	if _, err := io.Copy(tmpFile, r); err != nil {
		return fmt.Errorf("failed to write backup data: %w", err)
	}

	// Sync and seek to beginning
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek temporary file: %w", err)
	}

	// Create mongorestore command
	args := []string{
		"--uri", c.uri,
		"--archive=" + tmpFile.Name(),
		"--gzip",
		"--drop", // Drop collections before restoring
	}

	cmd := exec.CommandContext(ctx, "mongorestore", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mongorestore failed: %w", err)
	}

	return nil
}

// ListTables returns a list of all collections in the database
func (c *MongoDBConnector) ListTables(ctx context.Context) ([]string, error) {
	if c.uri == "" {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Create mongosh command to list collections
	query := `db.getCollectionNames().join('\n')`
	args := []string{
		"--uri", c.uri,
		"--quiet",
		"--eval", query,
	}

	cmd := exec.CommandContext(ctx, "mongosh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	// Parse output
	collections := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(collections) == 1 && collections[0] == "" {
		return []string{}, nil
	}

	return collections, nil
}

// GetInfo returns information about the database
func (c *MongoDBConnector) GetInfo(ctx context.Context) (map[string]string, error) {
	if c.uri == "" {
		return nil, fmt.Errorf("database connection not initialized")
	}

	info := make(map[string]string)

	// Get database stats
	statsQuery := `
		let stats = db.stats();
		let buildInfo = db.serverBuildInfo();
		print(JSON.stringify({
			size: stats.dataSize,
			storage_size: stats.storageSize,
			collections: stats.collections,
			objects: stats.objects,
			version: buildInfo.version
		}))
	`
	args := []string{
		"--uri", c.uri,
		"--quiet",
		"--eval", statsQuery,
	}

	cmd := exec.CommandContext(ctx, "mongosh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get database info: %w", err)
	}

	// Parse JSON output
	var stats struct {
		Size        int64  `json:"size"`
		StorageSize int64  `json:"storage_size"`
		Collections int    `json:"collections"`
		Objects     int64  `json:"objects"`
		Version     string `json:"version"`
	}

	if err := json.Unmarshal(output, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse database info: %w", err)
	}

	info["size"] = fmt.Sprintf("%d", stats.Size)
	info["storage_size"] = fmt.Sprintf("%d", stats.StorageSize)
	info["collections"] = fmt.Sprintf("%d", stats.Collections)
	info["objects"] = fmt.Sprintf("%d", stats.Objects)
	info["version"] = stats.Version

	return info, nil
}

// Type returns the database type
func (c *MongoDBConnector) Type() DBType {
	return MongoDB
} 