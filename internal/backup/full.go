package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/backyardBackup/internal/database"
	"github.com/yourusername/backyardBackup/internal/storage"
)

// FullBackup implements full database backup
type FullBackup struct {
	DB      database.Connector
	Storage storage.Provider
}

// FullMetadata contains metadata about a full backup
type FullMetadata struct {
	Tables    []string          `json:"tables"`
	DBInfo    map[string]string `json:"db_info"`
	Timestamp time.Time         `json:"timestamp"`
}

// NewFullBackup creates a new full backup instance
func NewFullBackup(db database.Connector, storage storage.Provider) *FullBackup {
	return &FullBackup{
		DB:      db,
		Storage: storage,
	}
}

// Backup performs a full database backup
func (b *FullBackup) Backup(ctx context.Context, opts BackupOptions) (*BackupResult, error) {
	if b.DB == nil {
		return nil, fmt.Errorf("database connector not initialized")
	}
	if b.Storage == nil {
		return nil, fmt.Errorf("storage provider not initialized")
	}

	// Start backup
	startTime := time.Now()
	backupID := uuid.New().String()

	// Get list of tables
	tables, err := b.DB.ListTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	// Filter tables based on options
	tables = filterTables(tables, opts.IncludeTables, opts.ExcludeTables)

	// Get database info
	dbInfo, err := b.DB.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database info: %w", err)
	}

	// Create metadata
	metadata := FullMetadata{
		Tables:    tables,
		DBInfo:    dbInfo,
		Timestamp: startTime,
	}

	// Create backup path
	backupPath := filepath.Join(
		string(b.DB.Type()),
		opts.SourceDB,
		"full",
		fmt.Sprintf("%s-%s.db", startTime.Format("20060102-150405"), backupID),
	)

	if opts.Compress {
		backupPath += ".gz"
	}

	// Create a pipe for streaming backup data
	pr, pw := io.Pipe()

	// Start backup in a goroutine
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		err := b.DB.Backup(ctx, pw, tables)
		if err != nil {
			errCh <- fmt.Errorf("failed to create backup: %w", err)
			return
		}
		errCh <- nil
	}()

	// Store backup data
	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	err = b.Storage.Store(ctx, backupPath, pr, map[string]string{
		"backup_type":   string(Full),
		"backup_id":     backupID,
		"source_db":     opts.SourceDB,
		"start_time":    startTime.Format(time.RFC3339),
		"metadata":      string(metadataStr),
		"is_compressed": fmt.Sprintf("%v", opts.Compress),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store backup: %w", err)
	}

	// Wait for backup to complete
	if err := <-errCh; err != nil {
		return nil, err
	}

	// Get backup size
	info, err := b.Storage.GetInfo(ctx, backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup info: %w", err)
	}

	result := &BackupResult{
		ID:           backupID,
		Type:         Full,
		StartTime:    startTime,
		EndTime:      time.Now(),
		Size:         info.Size,
		StoragePath:  backupPath,
		IsCompressed: opts.Compress,
		Success:      true,
	}

	return result, nil
}

// ListBackups returns a list of all available backups
func (b *FullBackup) ListBackups(ctx context.Context) ([]*BackupResult, error) {
	if b.Storage == nil {
		return nil, fmt.Errorf("storage provider not initialized")
	}

	// List all files in storage
	files, err := b.Storage.List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	var backups []*BackupResult
	for _, file := range files {
		// Skip non-backup files
		if !isBackupFile(file.Path) {
			continue
		}

		// Get backup metadata
		info, err := b.Storage.GetInfo(ctx, file.Path)
		if err != nil {
			continue
		}

		startTime, _ := time.Parse(time.RFC3339, info.Metadata["start_time"])
		isCompressed := info.Metadata["is_compressed"] == "true"

		backup := &BackupResult{
			ID:           info.Metadata["backup_id"],
			Type:         BackupType(info.Metadata["backup_type"]),
			StartTime:    startTime,
			EndTime:      info.LastModified,
			Size:         info.Size,
			StoragePath:  file.Path,
			IsCompressed: isCompressed,
			Success:      true,
		}
		backups = append(backups, backup)
	}

	return backups, nil
}

// GetBackup retrieves details about a specific backup
func (b *FullBackup) GetBackup(ctx context.Context, id string) (*BackupResult, error) {
	backups, err := b.ListBackups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	for _, backup := range backups {
		if backup.ID == id {
			return backup, nil
		}
	}

	return nil, fmt.Errorf("backup %s not found", id)
}

// DeleteBackup removes a backup from storage
func (b *FullBackup) DeleteBackup(ctx context.Context, id string) error {
	backup, err := b.GetBackup(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get backup: %w", err)
	}

	err = b.Storage.Delete(ctx, backup.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	return nil
}

// Helper functions
func isBackupFile(path string) bool {
	return strings.HasSuffix(path, ".db") || strings.HasSuffix(path, ".db.gz")
}

func filterTables(tables []string, include, exclude []string) []string {
	if len(include) == 0 && len(exclude) == 0 {
		return tables
	}

	// If include is specified, only keep those tables
	if len(include) > 0 {
		included := make(map[string]bool)
		for _, t := range include {
			included[t] = true
		}

		var filtered []string
		for _, t := range tables {
			if included[t] {
				filtered = append(filtered, t)
			}
		}
		tables = filtered
	}

	// If exclude is specified, remove those tables
	if len(exclude) > 0 {
		excluded := make(map[string]bool)
		for _, t := range exclude {
			excluded[t] = true
		}

		var filtered []string
		for _, t := range tables {
			if !excluded[t] {
				filtered = append(filtered, t)
			}
		}
		tables = filtered
	}

	return tables
} 