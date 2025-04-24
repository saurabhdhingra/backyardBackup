package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"
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

	// Validate options
	if opts.Type != Full {
		return nil, fmt.Errorf("invalid backup type: %s, expected full backup", opts.Type)
	}

	// Start backup
	startTime := time.Now()
	
	// Create backup ID
	backupID := uuid.New().String()
	
	// Create metadata
	metadata := map[string]string{
		"backup_id":   backupID,
		"backup_type": string(Full),
		"db_type":     string(b.DB.Type()),
		"start_time":  startTime.Format(time.RFC3339),
	}
	
	// Determine backup path
	dbInfo, err := b.DB.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database info: %w", err)
	}
	
	backupPath := filepath.Join(
		string(b.DB.Type()),
		dbInfo.Name,
		"full",
		fmt.Sprintf("%s-%s.db", startTime.Format("20060102-150405"), backupID),
	)
	
	if opts.Compress {
		backupPath += ".gz"
	}
	
	// Create a buffer to hold the backup data
	var buf bytes.Buffer
	var backupWriter io.Writer = &buf
	
	// Apply compression if needed
	var gzipWriter *gzip.Writer
	if opts.Compress {
		gzipWriter = gzip.NewWriter(&buf)
		backupWriter = gzipWriter
		metadata["compressed"] = "true"
	}
	
	// Perform the backup
	if err := b.DB.Backup(ctx, backupWriter, opts.IncludeTables); err != nil {
		return nil, fmt.Errorf("failed to backup database: %w", err)
	}
	
	// Close the gzip writer if used
	if gzipWriter != nil {
		if err := gzipWriter.Close(); err != nil {
			return nil, fmt.Errorf("failed to close gzip writer: %w", err)
		}
	}
	
	// Store the backup file
	if err := b.Storage.Store(ctx, backupPath, bytes.NewReader(buf.Bytes()), metadata); err != nil {
		return nil, fmt.Errorf("failed to store backup: %w", err)
	}
	
	// Create backup result
	endTime := time.Now()
	
	result := &BackupResult{
		ID:           backupID,
		Type:         Full,
		StartTime:    startTime,
		EndTime:      endTime,
		Size:         int64(buf.Len()),
		FileCount:    1,
		StoragePath:  backupPath,
		IsCompressed: opts.Compress,
		Success:      true,
	}
	
	return result, nil
}

// ListBackups returns a list of full backups
func (b *FullBackup) ListBackups(ctx context.Context) ([]*BackupResult, error) {
	if b.Storage == nil {
		return nil, fmt.Errorf("storage provider not initialized")
	}
	
	// List all files in the backup directory
	dbType := string(b.DB.Type())
	files, err := b.Storage.List(ctx, dbType)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}
	
	var backups []*BackupResult
	for _, file := range files {
		// Skip directories
		if file.IsDirectory {
			continue
		}
		
		// Skip non-backup files
		if filepath.Base(filepath.Dir(file.Path)) != "full" {
			continue
		}
		
		// Get backup metadata
		fileInfo, err := b.Storage.GetInfo(ctx, file.Path)
		if err != nil {
			continue
		}
		
		// Check if this is a full backup
		if fileInfo.Metadata["backup_type"] != string(Full) {
			continue
		}
		
		// Parse backup info from metadata
		backupID := fileInfo.Metadata["backup_id"]
		if backupID == "" {
			continue
		}
		
		startTimeStr := fileInfo.Metadata["start_time"]
		startTime, _ := time.Parse(time.RFC3339, startTimeStr)
		
		isCompressed := fileInfo.Metadata["compressed"] == "true"
		
		backup := &BackupResult{
			ID:           backupID,
			Type:         Full,
			StartTime:    startTime,
			EndTime:      fileInfo.LastModified,
			Size:         fileInfo.Size,
			FileCount:    1,
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
		return nil, err
	}
	
	for _, backup := range backups {
		if backup.ID == id {
			return backup, nil
		}
	}
	
	return nil, fmt.Errorf("backup not found: %s", id)
}

// DeleteBackup removes a backup from storage
func (b *FullBackup) DeleteBackup(ctx context.Context, id string) error {
	if b.Storage == nil {
		return fmt.Errorf("storage provider not initialized")
	}
	
	// Find the backup
	backup, err := b.GetBackup(ctx, id)
	if err != nil {
		return err
	}
	
	// Delete the backup file
	if err := b.Storage.Delete(ctx, backup.StoragePath); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}
	
	return nil
} 