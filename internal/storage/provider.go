package storage

import (
	"context"
	"io"
	"time"
)

// StorageType represents a storage provider type
type StorageType string

const (
	// Local file system storage
	Local StorageType = "local"
	// AWS S3 storage
	S3 StorageType = "s3"
	// Google Cloud Storage
	GCS StorageType = "gcs"
	// Azure Blob Storage
	Azure StorageType = "azure"
)

// ProviderConfig holds configuration for a storage provider
type ProviderConfig struct {
	Type      StorageType
	BasePath  string            // Used for all providers
	Bucket    string            // Used for cloud providers
	Region    string            // Used for cloud providers
	Endpoint  string            // Used for custom endpoints
	AccessKey string            // Used for authentication
	SecretKey string            // Used for authentication
	Options   map[string]string // Additional provider-specific options
}

// FileInfo contains metadata about a stored file
type FileInfo struct {
	Path         string
	Size         int64
	LastModified time.Time
	ContentType  string
	IsDirectory  bool
	Metadata     map[string]string
}

// Provider is the interface for storage operations
type Provider interface {
	// Initialize sets up the storage provider with the given configuration
	Initialize(ctx context.Context, config ProviderConfig) error
	
	// Store saves data from a reader to the given path
	Store(ctx context.Context, path string, r io.Reader, metadata map[string]string) error
	
	// Retrieve retrieves data from the given path and writes it to the writer
	Retrieve(ctx context.Context, path string, w io.Writer) error
	
	// Delete removes the file at the given path
	Delete(ctx context.Context, path string) error
	
	// List returns a list of files matching the given prefix
	List(ctx context.Context, prefix string) ([]FileInfo, error)
	
	// GetInfo returns metadata about the file at the given path
	GetInfo(ctx context.Context, path string) (*FileInfo, error)
	
	// Type returns the storage provider type
	Type() StorageType
} 