package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// AzureProvider implements the Provider interface for Azure Blob Storage
type AzureProvider struct {
	containerClient *azblob.ContainerClient
	config         ProviderConfig
}

// Initialize sets up the Azure storage provider
func (p *AzureProvider) Initialize(ctx context.Context, config ProviderConfig) error {
	if config.Type != Azure {
		return fmt.Errorf("invalid provider type: %s, expected: %s", config.Type, Azure)
	}

	// Validate required fields
	if config.Bucket == "" {
		return fmt.Errorf("container name is required for Azure storage provider")
	}
	if config.AccessKey == "" {
		return fmt.Errorf("connection string or access key is required for Azure storage provider")
	}

	// Create credential
	credential, err := azblob.NewSharedKeyCredential("", config.AccessKey)
	if err != nil {
		return fmt.Errorf("failed to create Azure credential: %w", err)
	}

	// Create pipeline
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// Create service URL
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.blob.core.windows.net", config.Bucket)
	}

	// Parse the URL
	serviceURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse Azure endpoint URL: %w", err)
	}

	// Create container client
	containerClient := azblob.NewContainerClient(serviceURL.String(), pipeline)

	// Check if container exists
	_, err = containerClient.GetProperties(ctx)
	if err != nil {
		return fmt.Errorf("failed to access container: %w", err)
	}

	p.containerClient = containerClient
	p.config = config

	return nil
}

// Store saves data from a reader to the given path
func (p *AzureProvider) Store(ctx context.Context, path string, r io.Reader, metadata map[string]string) error {
	if p.containerClient == nil {
		return fmt.Errorf("azure storage provider not initialized")
	}

	// Create blob client
	blobClient := p.containerClient.NewBlobClient(path)

	// Convert metadata to Azure format
	azureMetadata := make(map[string]string)
	for k, v := range metadata {
		azureMetadata[k] = v
	}

	// Upload data
	_, err := blobClient.Upload(ctx, r, &azblob.UploadOptions{
		Metadata: azureMetadata,
	})
	if err != nil {
		return fmt.Errorf("failed to upload blob: %w", err)
	}

	return nil
}

// Retrieve retrieves data from the given path and writes it to the writer
func (p *AzureProvider) Retrieve(ctx context.Context, path string, w io.Writer) error {
	if p.containerClient == nil {
		return fmt.Errorf("azure storage provider not initialized")
	}

	// Create blob client
	blobClient := p.containerClient.NewBlobClient(path)

	// Download data
	response, err := blobClient.Download(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to download blob: %w", err)
	}

	// Copy data to writer
	reader := response.Body(azblob.RetryReaderOptions{})
	defer reader.Close()

	if _, err := io.Copy(w, reader); err != nil {
		return fmt.Errorf("failed to read blob data: %w", err)
	}

	return nil
}

// Delete removes the file at the given path
func (p *AzureProvider) Delete(ctx context.Context, path string) error {
	if p.containerClient == nil {
		return fmt.Errorf("azure storage provider not initialized")
	}

	// Create blob client
	blobClient := p.containerClient.NewBlobClient(path)

	// Delete blob
	_, err := blobClient.Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}

	return nil
}

// List returns a list of files matching the given prefix
func (p *AzureProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	if p.containerClient == nil {
		return nil, fmt.Errorf("azure storage provider not initialized")
	}

	var files []FileInfo

	// List blobs
	pager := p.containerClient.ListBlobsFlat(&azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})

	for pager.NextPage(ctx) {
		resp := pager.PageResponse()

		for _, blob := range resp.Segment.BlobItems {
			// Get blob properties
			blobClient := p.containerClient.NewBlobClient(*blob.Name)
			props, err := blobClient.GetProperties(ctx, nil)
			if err != nil {
				continue
			}

			file := FileInfo{
				Path:         *blob.Name,
				Size:         *blob.Properties.ContentLength,
				LastModified: *blob.Properties.LastModified,
				ContentType:  *blob.Properties.ContentType,
				IsDirectory:  strings.HasSuffix(*blob.Name, "/"),
				Metadata:     props.Metadata,
			}
			files = append(files, file)
		}
	}

	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("failed to list blobs: %w", err)
	}

	return files, nil
}

// GetInfo returns metadata about the file at the given path
func (p *AzureProvider) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	if p.containerClient == nil {
		return nil, fmt.Errorf("azure storage provider not initialized")
	}

	// Create blob client
	blobClient := p.containerClient.NewBlobClient(path)

	// Get blob properties
	props, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob properties: %w", err)
	}

	info := &FileInfo{
		Path:         path,
		Size:         props.ContentLength,
		LastModified: props.LastModified,
		ContentType:  props.ContentType,
		IsDirectory:  strings.HasSuffix(path, "/"),
		Metadata:     props.Metadata,
	}

	return info, nil
}

// Type returns the storage provider type
func (p *AzureProvider) Type() StorageType {
	return Azure
}

// Close closes any resources held by the provider
func (p *AzureProvider) Close() error {
	// No resources to close for Azure
	return nil
} 