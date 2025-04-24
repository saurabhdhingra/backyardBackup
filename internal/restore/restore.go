package restore

import (
	"context"
	"time"
)

// RestoreOptions contains configuration for a restore operation
type RestoreOptions struct {
	BackupID       string
	TargetDB       string
	IncludeTables  []string
	ExcludeTables  []string
	PointInTime    time.Time // For point-in-time recovery
	OverwriteExisting bool
}

// RestoreResult contains information about a completed restore
type RestoreResult struct {
	ID             string
	BackupID       string
	StartTime      time.Time
	EndTime        time.Time
	TablesRestored []string
	Success        bool
	ErrorMessage   string
}

// Restorer is the interface for database restore operations
type Restorer interface {
	// Restore performs a database restore according to the provided options
	Restore(ctx context.Context, opts RestoreOptions) (*RestoreResult, error)
	
	// ValidateBackup checks if a backup is valid and can be restored
	ValidateBackup(ctx context.Context, backupID string) (bool, error)
	
	// ListRestores returns a list of all restore operations
	ListRestores(ctx context.Context) ([]*RestoreResult, error)
	
	// GetRestore retrieves details about a specific restore operation
	GetRestore(ctx context.Context, id string) (*RestoreResult, error)
} 