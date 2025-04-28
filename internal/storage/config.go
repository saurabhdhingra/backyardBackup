package storage

import "time"

// ProviderConfig holds the configuration for storage providers
type ProviderConfig struct {
	// GCS configuration
	GCS *GCSConfig
	// Add other provider configs here as needed
}

// GCSConfig holds the configuration for Google Cloud Storage
type GCSConfig struct {
	// CredentialsFile is the path to the service account credentials JSON file
	CredentialsFile string
	// BucketName is the name of the GCS bucket
	BucketName string
	// Prefix is the optional prefix for all objects in the bucket
	Prefix string
}

// FileInfo represents information about a file in storage
type FileInfo struct {
	// Path is the relative path of the file
	Path string
	// Size is the size of the file in bytes
	Size int64
	// LastModified is the last modification time of the file
	LastModified time.Time
} 