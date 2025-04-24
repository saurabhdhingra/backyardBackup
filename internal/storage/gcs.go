package storage

import (
	"context"
	"fmt"
	"io"
)

// GCSProvider implements the Provider interface for Google Cloud Storage
type GCSProvider struct {
	// Add GCS specific fields here when implementing
}

// Initialize sets up the GCS storage provider
func (p *GCSProvider) Initialize(_ context.Context, config ProviderConfig) error {
	if config.Type != GCS {
		return fmt.Errorf("invalid provider type: %s, expected: %s", config.Type, GCS)
	}
	
	// Validate required fields
	if config.Bucket == "" {
		return fmt.Errorf("bucket is required for GCS storage provider")
	}
	
	return fmt.Errorf("GCS storage provider not implemented yet")
}

// Store saves data from a reader to the given path
func (p *GCSProvider) Store(_ context.Context, path string, r io.Reader, metadata map[string]string) error {
	return fmt.Errorf("GCS storage provider not implemented yet")
}

// Retrieve retrieves data from the given path and writes it to the writer
func (p *GCSProvider) Retrieve(_ context.Context, path string, w io.Writer) error {
	return fmt.Errorf("GCS storage provider not implemented yet")
}

// Delete removes the file at the given path
func (p *GCSProvider) Delete(_ context.Context, path string) error {
	return fmt.Errorf("GCS storage provider not implemented yet")
}

// List returns a list of files matching the given prefix
func (p *GCSProvider) List(_ context.Context, prefix string) ([]FileInfo, error) {
	return nil, fmt.Errorf("GCS storage provider not implemented yet")
}

// GetInfo returns metadata about the file at the given path
func (p *GCSProvider) GetInfo(_ context.Context, path string) (*FileInfo, error) {
	return nil, fmt.Errorf("GCS storage provider not implemented yet")
}

// Type returns the storage provider type
func (p *GCSProvider) Type() StorageType {
	return GCS
} 