package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSProvider implements the Provider interface for Google Cloud Storage
type GCSProvider struct {
	client     *storage.Client
	bucketName string
	prefix     string
}

// NewGCSProvider creates a new GCS storage provider
func NewGCSProvider(ctx context.Context, config *ProviderConfig) (*GCSProvider, error) {
	if config.GCS == nil {
		return nil, fmt.Errorf("GCS configuration is required")
	}

	client, err := storage.NewClient(ctx, option.WithCredentialsFile(config.GCS.CredentialsFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSProvider{
		client:     client,
		bucketName: config.GCS.BucketName,
		prefix:     config.GCS.Prefix,
	}, nil
}

// Store stores data in GCS
func (p *GCSProvider) Store(ctx context.Context, path string, data io.Reader) error {
	bucket := p.client.Bucket(p.bucketName)
	obj := bucket.Object(filepath.Join(p.prefix, path))
	writer := obj.NewWriter(ctx)

	if _, err := io.Copy(writer, data); err != nil {
		writer.Close()
		return fmt.Errorf("failed to copy data to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	return nil
}

// Retrieve retrieves data from GCS
func (p *GCSProvider) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	bucket := p.client.Bucket(p.bucketName)
	obj := bucket.Object(filepath.Join(p.prefix, path))
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS reader: %w", err)
	}

	return reader, nil
}

// Delete deletes a file from GCS
func (p *GCSProvider) Delete(ctx context.Context, path string) error {
	bucket := p.client.Bucket(p.bucketName)
	obj := bucket.Object(filepath.Join(p.prefix, path))
	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object from GCS: %w", err)
	}
	return nil
}

// List lists files in GCS with the given prefix
func (p *GCSProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	bucket := p.client.Bucket(p.bucketName)
	fullPrefix := filepath.Join(p.prefix, prefix)
	it := bucket.Objects(ctx, &storage.Query{Prefix: fullPrefix})

	var files []FileInfo
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in GCS: %w", err)
		}

		// Skip directories (objects ending with /)
		if strings.HasSuffix(attrs.Name, "/") {
			continue
		}

		// Remove the provider's prefix from the path
		relativePath := strings.TrimPrefix(attrs.Name, p.prefix)
		relativePath = strings.TrimPrefix(relativePath, "/")

		files = append(files, FileInfo{
			Path:         relativePath,
			Size:         attrs.Size,
			LastModified: attrs.Updated,
		})
	}

	return files, nil
}

// GetInfo gets information about a file in GCS
func (p *GCSProvider) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	bucket := p.client.Bucket(p.bucketName)
	obj := bucket.Object(filepath.Join(p.prefix, path))
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object attributes from GCS: %w", err)
	}

	relativePath := strings.TrimPrefix(attrs.Name, p.prefix)
	relativePath = strings.TrimPrefix(relativePath, "/")

	return &FileInfo{
		Path:         relativePath,
		Size:         attrs.Size,
		LastModified: attrs.Updated,
	}, nil
}

// Close closes the GCS client
func (p *GCSProvider) Close() error {
	return p.client.Close()
} 