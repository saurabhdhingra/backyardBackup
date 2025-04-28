package restore

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/backyardBackup/internal/backup"
	"github.com/yourusername/backyardBackup/internal/database"
	"github.com/yourusername/backyardBackup/internal/storage"
)

// SelectiveRestorer implements the Restorer interface for selective table restoration
type SelectiveRestorer struct {
	DB      database.Connector
	Storage storage.Provider
	Backups backup.Backuper
	restores map[string]*RestoreResult
}

// NewSelectiveRestorer creates a new selective restorer
func NewSelectiveRestorer(db database.Connector, storage storage.Provider, backups backup.Backuper) *SelectiveRestorer {
	return &SelectiveRestorer{
		DB:       db,
		Storage:  storage,
		Backups:  backups,
		restores: make(map[string]*RestoreResult),
	}
}

// Restore performs a selective database restore according to the provided options
func (r *SelectiveRestorer) Restore(ctx context.Context, opts RestoreOptions) (*RestoreResult, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database connector not initialized")
	}
	if r.Storage == nil {
		return nil, fmt.Errorf("storage provider not initialized")
	}
	if r.Backups == nil {
		return nil, fmt.Errorf("backup service not initialized")
	}

	// Get backup details
	backupInfo, err := r.Backups.GetBackup(ctx, opts.BackupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup info: %w", err)
	}

	// Create restore ID
	restoreID := uuid.New().String()

	// Create restore result
	result := &RestoreResult{
		ID:       restoreID,
		BackupID: opts.BackupID,
		StartTime: time.Now(),
	}

	// Store result
	r.restores[restoreID] = result

	// Create a pipe for streaming backup data
	pr, pw := io.Pipe()

	// Start retrieving backup data in a goroutine
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		err := r.Storage.Retrieve(ctx, backupInfo.StoragePath, pw)
		if err != nil {
			errCh <- fmt.Errorf("failed to retrieve backup: %w", err)
			return
		}
		errCh <- nil
	}()

	// Restore the database
	err = r.DB.Restore(ctx, pr)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("failed to restore database: %v", err)
		result.EndTime = time.Now()
		return result, fmt.Errorf("failed to restore database: %w", err)
	}

	// Wait for retrieval to complete
	if err := <-errCh; err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		result.EndTime = time.Now()
		return result, err
	}

	// Get list of restored tables
	tables, err := r.DB.ListTables(ctx)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("failed to list restored tables: %v", err)
		result.EndTime = time.Now()
		return result, fmt.Errorf("failed to list restored tables: %w", err)
	}

	result.TablesRestored = tables
	result.Success = true
	result.EndTime = time.Now()

	return result, nil
}

// ValidateBackup checks if a backup is valid and can be restored
func (r *SelectiveRestorer) ValidateBackup(ctx context.Context, backupID string) (bool, error) {
	if r.Backups == nil {
		return false, fmt.Errorf("backup service not initialized")
	}

	// Get backup details
	backup, err := r.Backups.GetBackup(ctx, backupID)
	if err != nil {
		return false, fmt.Errorf("failed to get backup info: %w", err)
	}

	// Check if backup exists and was successful
	if !backup.Success {
		return false, nil
	}

	// Check if backup file exists in storage
	_, err = r.Storage.GetInfo(ctx, backup.StoragePath)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// ListRestores returns a list of all restore operations
func (r *SelectiveRestorer) ListRestores(ctx context.Context) ([]*RestoreResult, error) {
	var results []*RestoreResult
	for _, restore := range r.restores {
		results = append(results, restore)
	}
	return results, nil
}

// GetRestore retrieves details about a specific restore operation
func (r *SelectiveRestorer) GetRestore(ctx context.Context, id string) (*RestoreResult, error) {
	result, ok := r.restores[id]
	if !ok {
		return nil, fmt.Errorf("restore operation %s not found", id)
	}
	return result, nil
} 