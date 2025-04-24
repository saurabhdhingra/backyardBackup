package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalProvider implements the Provider interface for local filesystem storage
type LocalProvider struct {
	basePath string
}

// Initialize sets up the local storage provider
func (p *LocalProvider) Initialize(_ context.Context, config ProviderConfig) error {
	if config.Type != Local {
		return fmt.Errorf("invalid provider type: %s, expected: %s", config.Type, Local)
	}

	p.basePath = config.BasePath
	if p.basePath == "" {
		return fmt.Errorf("base path is required for local storage provider")
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(p.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	return nil
}

// Store saves data from a reader to the given path
func (p *LocalProvider) Store(_ context.Context, path string, r io.Reader, metadata map[string]string) error {
	fullPath := filepath.Join(p.basePath, path)
	
	// Create parent directories if they don't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Create the file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	// Copy data from reader to file
	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	// If metadata is provided, store it in a sidecar file
	if len(metadata) > 0 {
		metadataPath := fullPath + ".meta"
		if err := saveMetadata(metadataPath, metadata); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}
	}
	
	return nil
}

// Retrieve retrieves data from the given path and writes it to the writer
func (p *LocalProvider) Retrieve(_ context.Context, path string, w io.Writer) error {
	fullPath := filepath.Join(p.basePath, path)
	
	// Open the file
	file, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Copy data from file to writer
	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	return nil
}

// Delete removes the file at the given path
func (p *LocalProvider) Delete(_ context.Context, path string) error {
	fullPath := filepath.Join(p.basePath, path)
	
	// Remove the file
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	
	// Also remove the metadata file if it exists
	metadataPath := fullPath + ".meta"
	os.Remove(metadataPath) // Ignore error if metadata file doesn't exist
	
	return nil
}

// List returns a list of files matching the given prefix
func (p *LocalProvider) List(_ context.Context, prefix string) ([]FileInfo, error) {
	prefixPath := filepath.Join(p.basePath, prefix)
	
	// Get the parent directory of the prefix
	var dir string
	if info, err := os.Stat(prefixPath); err == nil && info.IsDir() {
		dir = prefixPath
	} else {
		dir = filepath.Dir(prefixPath)
	}
	
	// List all files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}
	
	var files []FileInfo
	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())
		
		// Skip if the file doesn't match the prefix
		if prefix != "" && !filepath.HasPrefix(entryPath, prefixPath) {
			continue
		}
		
		// Skip metadata files
		if filepath.Ext(entryPath) == ".meta" {
			continue
		}
		
		// Get file info
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		// Get relative path from basePath
		relPath, err := filepath.Rel(p.basePath, entryPath)
		if err != nil {
			continue
		}
		
		// Load metadata if available
		metadata, _ := loadMetadata(entryPath + ".meta")
		
		fileInfo := FileInfo{
			Path:         relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			IsDirectory:  info.IsDir(),
			Metadata:     metadata,
		}
		
		files = append(files, fileInfo)
	}
	
	return files, nil
}

// GetInfo returns metadata about the file at the given path
func (p *LocalProvider) GetInfo(_ context.Context, path string) (*FileInfo, error) {
	fullPath := filepath.Join(p.basePath, path)
	
	// Get file info
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	
	// Load metadata if available
	metadata, _ := loadMetadata(fullPath + ".meta")
	
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

// Helper functions for metadata

func saveMetadata(path string, metadata map[string]string) error {
	// Simple implementation that writes metadata to a text file
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	for key, value := range metadata {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, value); err != nil {
			return err
		}
	}
	
	return nil
}

func loadMetadata(path string) (map[string]string, error) {
	metadata := make(map[string]string)
	
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return metadata, nil
	}
	
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return metadata, err
	}
	defer file.Close()
	
	// Read line by line
	var key, value string
	for {
		if _, err := fmt.Fscanf(file, "%s=%s\n", &key, &value); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		metadata[key] = value
	}
	
	return metadata, nil
} 