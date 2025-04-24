package backup

import (
	"context"
	"time"
)

// BackupType represents the type of backup
type BackupType string

const (
	// Full backup type
	Full BackupType = "full"
	// Incremental backup type
	Incremental BackupType = "incremental"
	// Differential backup type
	Differential BackupType = "differential"
)

// BackupResult contains information about a completed backup
type BackupResult struct {
	ID           string
	Type         BackupType
	StartTime    time.Time
	EndTime      time.Time
	Size         int64
	FileCount    int
	StoragePath  string
	IsCompressed bool
	Success      bool
	ErrorMessage string
}

// BackupOptions contains configuration for a backup operation
type BackupOptions struct {
	Type         BackupType
	Compress     bool
	SourceDB     string
	DestStorage  string
	ExcludeTables []string
	IncludeTables []string
	MaxSize      int64
}

// Backuper is the interface for database backup operations
type Backuper interface {
	// Backup performs a database backup according to the provided options
	Backup(ctx context.Context, opts BackupOptions) (*BackupResult, error)
	
	// ListBackups returns a list of all available backups
	ListBackups(ctx context.Context) ([]*BackupResult, error)
	
	// GetBackup retrieves details about a specific backup
	GetBackup(ctx context.Context, id string) (*BackupResult, error)
	
	// DeleteBackup removes a backup from storage
	DeleteBackup(ctx context.Context, id string) error
} 