package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// AzureProvider implements the Provider interface for Azure Blob Storage
type AzureProvider struct {
	containerURL azblob.ContainerURL
	credential   *azblob.SharedKeyCredential
	config      ProviderConfig
}

// Initialize sets up the Azure storage provider
func (p *AzureProvider) Initialize(ctx context.Context, config ProviderConfig) error {
	if config.Type != Azure {
		return fmt.Errorf("invalid provider type: %s, expected: %s", config.Type, Azure)
	}
	
	// Validate required fields
	if config.Bucket == "" {
		return fmt.Errorf("bucket (container) is required for Azure storage provider")
	}
	if config.AccessKey == "" {
		return fmt.Errorf("access key is required for Azure storage provider")
	}
	if config.SecretKey == "" {
		return fmt.Errorf("secret key is required for Azure storage provider")
	}

	// Create shared key credential
	credential, err := azblob.NewSharedKeyCredential(config.AccessKey, config.SecretKey)
	if err != nil {
		return fmt.Errorf("failed to create shared key credential: %w", err)
	}

	// Create pipeline
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// Build service URL
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.blob.core.windows.net", config.AccessKey)
	}

	// Create container URL
	serviceURL := azblob.NewServiceURL(*parseURL(endpoint), pipeline)
	containerURL := serviceURL.NewContainerURL(config.Bucket)

	// Create container if it doesn't exist
	_, err = containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
	if err != nil {
		// Ignore error if container already exists
		if !isContainerAlreadyExists(err) {
			return fmt.Errorf("failed to create container: %w", err)
		}
	}

	p.containerURL = containerURL
	p.credential = credential
	p.config = config

	return nil
}

// Store saves data from a reader to the given path
func (p *AzureProvider) Store(ctx context.Context, path string, r io.Reader, metadata map[string]string) error {
	if p.containerURL.String() == "" {
		return fmt.Errorf("azure storage provider not initialized")
	}

	// Create blob URL
	blobURL := p.containerURL.NewBlockBlobURL(path)

	// Upload the blob
	_, err := azblob.UploadStreamToBlockBlob(ctx, r, blobURL,
		azblob.UploadStreamToBlockBlobOptions{
			BufferSize: 4 * 1024 * 1024, // 4MB buffer
			MaxBuffers: 16,
			Metadata:   metadata,
		})
	if err != nil {
		return fmt.Errorf("failed to upload blob: %w", err)
	}

	return nil
}

// Retrieve retrieves data from the given path and writes it to the writer
func (p *AzureProvider) Retrieve(ctx context.Context, path string, w io.Writer) error {
	if p.containerURL.String() == "" {
		return fmt.Errorf("azure storage provider not initialized")
	}

	// Create blob URL
	blobURL := p.containerURL.NewBlockBlobURL(path)

	// Download the blob
	response, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return fmt.Errorf("failed to download blob: %w", err)
	}

	// Read the data
	reader := response.Body(azblob.RetryReaderOptions{MaxRetryRequests: 3})
	defer reader.Close()

	_, err = io.Copy(w, reader)
	if err != nil {
		return fmt.Errorf("failed to read blob data: %w", err)
	}

	return nil
}

// Delete removes the file at the given path
func (p *AzureProvider) Delete(ctx context.Context, path string) error {
	if p.containerURL.String() == "" {
		return fmt.Errorf("azure storage provider not initialized")
	}

	// Create blob URL
	blobURL := p.containerURL.NewBlockBlobURL(path)

	// Delete the blob
	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}

	return nil
}

// List returns a list of files matching the given prefix
func (p *AzureProvider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	if p.containerURL.String() == "" {
		return nil, fmt.Errorf("azure storage provider not initialized")
	}

	var files []FileInfo

	// List the blobs
	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlob, err := p.containerURL.ListBlobsFlatSegment(ctx, marker,
			azblob.ListBlobsSegmentOptions{
				Prefix: prefix,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs: %w", err)
		}

		marker = listBlob.NextMarker

		// Process each blob
		for _, blobInfo := range listBlob.Segment.BlobItems {
			file := FileInfo{
				Path:         blobInfo.Name,
				Size:         *blobInfo.Properties.ContentLength,
				LastModified: blobInfo.Properties.LastModified,
				ContentType:  *blobInfo.Properties.ContentType,
				IsDirectory:  strings.HasSuffix(blobInfo.Name, "/"),
				Metadata:     blobInfo.Metadata,
			}
			files = append(files, file)
		}
	}

	return files, nil
}

// GetInfo returns metadata about the file at the given path
func (p *AzureProvider) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	if p.containerURL.String() == "" {
		return nil, fmt.Errorf("azure storage provider not initialized")
	}

	// Create blob URL
	blobURL := p.containerURL.NewBlockBlobURL(path)

	// Get blob properties
	props, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get blob properties: %w", err)
	}

	info := &FileInfo{
		Path:         path,
		Size:         props.ContentLength(),
		LastModified: props.LastModified(),
		ContentType:  props.ContentType(),
		IsDirectory:  strings.HasSuffix(path, "/"),
		Metadata:     props.NewMetadata(),
	}

	return info, nil
}

// Type returns the storage provider type
func (p *AzureProvider) Type() StorageType {
	return Azure
}

// Helper functions
func parseURL(endpoint string) *azblob.URL {
	url, _ := azblob.ParseURL(endpoint)
	return url
}

func isContainerAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	if serr, ok := err.(azblob.StorageError); ok {
		return serr.ServiceCode() == azblob.ServiceCodeContainerAlreadyExists
	}
	return false
} 