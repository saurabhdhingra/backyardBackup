package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSProvider implements the Provider interface for Google Cloud Storage
type GCSProvider struct {
	client  *storage.Client
	bucket  *storage.BucketHandle
	config  ProviderConfig
}

// Initialize sets up the GCS storage provider
func (p *GCSProvider) Initialize(ctx context.Context, config ProviderConfig) error {
	if config.Type != GCS {
		return fmt.Errorf("invalid provider type: %s, expected: %s", config.Type, GCS)
	}

	// Validate required fields
	if config.Bucket == "" {
		return fmt.Errorf("bucket is required for GCS storage provider")
	}

	var opts []option.ClientOption
	if config.AccessKey != "" {
		// If credentials are provided directly
		opts = append(opts, option.WithCredentialsJSON([]byte(config.AccessKey)))
	}
	if config.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(config.Endpoint))
	}

	// Create GCS client
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}

	// Get bucket handle
	bucket := client.Bucket(config.Bucket)

	// Check if bucket exists
	_, err = bucket.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to access bucket: %w", err)
	}

	p.client = client
	p.bucket = bucket
	p.config = config

	return nil
}

// Store saves data from a reader to the given path
func (p *GCSProvider) Store(ctx context.Context, path string, r io.Reader, metadata map[string]string) error {
	if p.client == nil {
		return fmt.Errorf("gcs storage provider not initialized")
	}

	// Create object handle
	obj := p.bucket.Object(path)

	// Create writer
	writer := obj.NewWriter(ctx)
	
	// Set metadata
	writer.Metadata = metadata

	// Copy data
	if _, err := io.Copy(writer, r); err != nil {
		writer.Close()
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Close writer
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	return nil
}

// Retrieve retrieves data from the given path and writes it to the writer
func (p *GCSProvider) Retrieve(ctx context.Context, path string, w io.Writer) error {
	if p.client == nil {
		return fmt.Errorf("gcs storage provider not initialized")
	}

	// Create object handle
	obj := p.bucket.Object(path)

	// Create reader
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	// Copy data
	if _, err := io.Copy(w, reader); err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	return nil
}

// Delete removes the file at the given path
func (p *GCSProvider) Delete(ctx context.Context, path string) error {
	if p.client == nil {
		return fmt.Errorf("gcs storage provider not initialized")
	}

	// Create object handle and delete
	obj := p.bucket.Object(path)
	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// List returns a list of files matching the given prefix
func (p *GCSProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("gcs storage provider not initialized")
	}

	var files []FileInfo

	// Create iterator
	it := p.bucket.Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	// Iterate through objects
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		file := FileInfo{
			Path:         attrs.Name,
			Size:         attrs.Size,
			LastModified: attrs.Updated,
			ContentType:  attrs.ContentType,
			IsDirectory:  strings.HasSuffix(attrs.Name, "/"),
			Metadata:     attrs.Metadata,
		}
		files = append(files, file)
	}

	return files, nil
}

// GetInfo returns metadata about the file at the given path
func (p *GCSProvider) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("gcs storage provider not initialized")
	}

	// Get object attributes
	attrs, err := p.bucket.Object(path).Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	info := &FileInfo{
		Path:         attrs.Name,
		Size:         attrs.Size,
		LastModified: attrs.Updated,
		ContentType:  attrs.ContentType,
		IsDirectory:  strings.HasSuffix(attrs.Name, "/"),
		Metadata:     attrs.Metadata,
	}

	return info, nil
}

// Type returns the storage provider type
func (p *GCSProvider) Type() StorageType {
	return GCS
}

// Close closes the GCS client
func (p *GCSProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
} 