package storage

import (
	"context"
	"fmt"
	"io"
)

// AzureProvider implements the Provider interface for Azure Blob Storage
type AzureProvider struct {
	// Add Azure specific fields here when implementing
}

// Initialize sets up the Azure storage provider
func (p *AzureProvider) Initialize(_ context.Context, config ProviderConfig) error {
	if config.Type != Azure {
		return fmt.Errorf("invalid provider type: %s, expected: %s", config.Type, Azure)
	}
	
	// Validate required fields
	if config.Bucket == "" {
		return fmt.Errorf("bucket (container) is required for Azure storage provider")
	}
	
	return fmt.Errorf("Azure storage provider not implemented yet")
}

// Store saves data from a reader to the given path
func (p *AzureProvider) Store(_ context.Context, path string, r io.Reader, metadata map[string]string) error {
	return fmt.Errorf("Azure storage provider not implemented yet")
}

// Retrieve retrieves data from the given path and writes it to the writer
func (p *AzureProvider) Retrieve(_ context.Context, path string, w io.Writer) error {
	return fmt.Errorf("Azure storage provider not implemented yet")
}

// Delete removes the file at the given path
func (p *AzureProvider) Delete(_ context.Context, path string) error {
	return fmt.Errorf("Azure storage provider not implemented yet")
}

// List returns a list of files matching the given prefix
func (p *AzureProvider) List(_ context.Context, prefix string) ([]FileInfo, error) {
	return nil, fmt.Errorf("Azure storage provider not implemented yet")
}

// GetInfo returns metadata about the file at the given path
func (p *AzureProvider) GetInfo(_ context.Context, path string) (*FileInfo, error) {
	return nil, fmt.Errorf("Azure storage provider not implemented yet")
}

// Type returns the storage provider type
func (p *AzureProvider) Type() StorageType {
	return Azure
} 