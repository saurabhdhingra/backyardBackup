package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalProvider implements the Provider interface for local filesystem storage
type LocalProvider struct {
	basePath string
	config   ProviderConfig
}

// Initialize sets up the local storage provider
func (p *LocalProvider) Initialize(ctx context.Context, config ProviderConfig) error {
	if config.Type != Local {
		return fmt.Errorf("invalid provider type: %s, expected: %s", config.Type, Local)
	}

	// Validate required fields
	if config.BasePath == "" {
		return fmt.Errorf("base path is required for local storage provider")
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	p.basePath = config.BasePath
	p.config = config

	return nil
}

// Store saves data from a reader to the given path
func (p *LocalProvider) Store(ctx context.Context, path string, r io.Reader, metadata map[string]string) error {
	if p.basePath == "" {
		return fmt.Errorf("local storage provider not initialized")
	}

	// Create full path
	fullPath := filepath.Join(p.basePath, path)

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Store metadata in a separate file if provided
	if len(metadata) > 0 {
		metadataPath := fullPath + ".metadata"
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
			return fmt.Errorf("failed to write metadata: %w", err)
		}
	}

	return nil
}

// Retrieve retrieves data from the given path and writes it to the writer
func (p *LocalProvider) Retrieve(ctx context.Context, path string, w io.Writer) error {
	if p.basePath == "" {
		return fmt.Errorf("local storage provider not initialized")
	}

	// Open file
	file, err := os.Open(filepath.Join(p.basePath, path))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	return nil
}

// Delete removes the file at the given path
func (p *LocalProvider) Delete(ctx context.Context, path string) error {
	if p.basePath == "" {
		return fmt.Errorf("local storage provider not initialized")
	}

	fullPath := filepath.Join(p.basePath, path)

	// Delete file
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata file if it exists
	metadataPath := fullPath + ".metadata"
	os.Remove(metadataPath) // Ignore error if metadata file doesn't exist

	return nil
}

// List returns a list of files matching the given prefix
func (p *LocalProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	if p.basePath == "" {
		return nil, fmt.Errorf("local storage provider not initialized")
	}

	var files []FileInfo

	// Walk through the directory
	err := filepath.Walk(p.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip metadata files
		if strings.HasSuffix(path, ".metadata") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(p.basePath, path)
		if err != nil {
			return err
		}

		// Check prefix
		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		// Get metadata if available
		var metadata map[string]string
		metadataPath := path + ".metadata"
		if metadataBytes, err := os.ReadFile(metadataPath); err == nil {
			if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
				metadata = make(map[string]string)
			}
		}

		file := FileInfo{
			Path:         relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			IsDirectory:  info.IsDir(),
			Metadata:     metadata,
		}

		if !info.IsDir() {
			files = append(files, file)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// GetInfo returns metadata about the file at the given path
func (p *LocalProvider) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	if p.basePath == "" {
		return nil, fmt.Errorf("local storage provider not initialized")
	}

	fullPath := filepath.Join(p.basePath, path)

	// Get file info
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get metadata if available
	var metadata map[string]string
	metadataPath := fullPath + ".metadata"
	if metadataBytes, err := os.ReadFile(metadataPath); err == nil {
		if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
			metadata = make(map[string]string)
		}
	}

	fileInfo := &FileInfo{
		Path:         path,
		Size:         info.Size(),
		LastModified: info.ModTime(),
		IsDirectory:  info.IsDir(),
		Metadata:     metadata,
	}

	return fileInfo, nil
}

// Type returns the storage provider type
func (p *LocalProvider) Type() StorageType {
	return Local
}

// Close closes any resources held by the provider
func (p *LocalProvider) Close() error {
	// No resources to close for local storage
	return nil
} 