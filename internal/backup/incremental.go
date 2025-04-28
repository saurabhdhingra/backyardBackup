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

// IncrementalBackup implements incremental database backup
type IncrementalBackup struct {
	DB      database.Connector
	Storage storage.Provider
}

// IncrementalMetadata contains metadata about an incremental backup
type IncrementalMetadata struct {
	BaseBackupID string            `json:"base_backup_id"`
	Changes      map[string]string `json:"changes"` // table -> checksum
	Timestamp    time.Time         `json:"timestamp"`
}

// NewIncrementalBackup creates a new incremental backup instance
func NewIncrementalBackup(db database.Connector, storage storage.Provider) *IncrementalBackup {
	return &IncrementalBackup{
		DB:      db,
		Storage: storage,
	}
}

// findLatestFullBackup finds the most recent full backup
func (b *IncrementalBackup) findLatestFullBackup(ctx context.Context, dbName string) (*BackupResult, error) {
	// List all backups
	backups, err := b.ListBackups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	var latest *BackupResult
	for _, backup := range backups {
		if backup.Type == Full {
			if latest == nil || backup.StartTime.After(latest.StartTime) {
				latest = backup
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no full backup found for database %s", dbName)
	}

	return latest, nil
}

// Backup performs an incremental database backup
func (b *IncrementalBackup) Backup(ctx context.Context, opts BackupOptions) (*BackupResult, error) {
	if b.DB == nil {
		return nil, fmt.Errorf("database connector not initialized")
	}
	if b.Storage == nil {
		return nil, fmt.Errorf("storage provider not initialized")
	}

	// Find latest full backup
	baseBackup, err := b.findLatestFullBackup(ctx, opts.SourceDB)
	if err != nil {
		return nil, fmt.Errorf("failed to find base backup: %w", err)
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

	// Create metadata
	metadata := IncrementalMetadata{
		BaseBackupID: baseBackup.ID,
		Changes:      make(map[string]string),
		Timestamp:    startTime,
	}

	// Create backup path
	backupPath := filepath.Join(
		string(b.DB.Type()),
		opts.SourceDB,
		"incremental",
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
		"backup_type":   string(Incremental),
		"base_backup":   baseBackup.ID,
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
		Type:         Incremental,
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
func (b *IncrementalBackup) ListBackups(ctx context.Context) ([]*BackupResult, error) {
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
func (b *IncrementalBackup) GetBackup(ctx context.Context, id string) (*BackupResult, error) {
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
func (b *IncrementalBackup) DeleteBackup(ctx context.Context, id string) error {
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