package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Provider implements the Provider interface for Amazon S3
type S3Provider struct {
	client *s3.Client
	bucket string
	config ProviderConfig
}

// Initialize sets up the S3 storage provider
func (p *S3Provider) Initialize(ctx context.Context, config ProviderConfig) error {
	if config.Type != S3 {
		return fmt.Errorf("invalid provider type: %s, expected: %s", config.Type, S3)
	}

	// Validate required fields
	if config.Bucket == "" {
		return fmt.Errorf("bucket is required for S3 storage provider")
	}

	// Create AWS config
	var opts []func(*config.LoadOptions) error

	// If credentials are provided directly
	if config.AccessKey != "" && config.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKey,
			config.SecretKey,
			"",
		)))
	}

	// If region is specified
	if config.Region != "" {
		opts = append(opts, config.WithRegion(config.Region))
	}

	// If endpoint is specified (for S3-compatible services)
	if config.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           config.Endpoint,
				SigningRegion: region,
			}, nil
		})
		opts = append(opts, config.WithEndpointResolverWithOptions(customResolver))
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	// Check if bucket exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(config.Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to access bucket: %w", err)
	}

	p.client = client
	p.bucket = config.Bucket
	p.config = config

	return nil
}

// Store saves data from a reader to the given path
func (p *S3Provider) Store(ctx context.Context, path string, r io.Reader, metadata map[string]string) error {
	if p.client == nil {
		return fmt.Errorf("s3 storage provider not initialized")
	}

	// Convert metadata to S3 format
	s3Metadata := make(map[string]string)
	for k, v := range metadata {
		s3Metadata[k] = v
	}

	// Upload object
	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(path),
		Body:     r,
		Metadata: s3Metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Retrieve retrieves data from the given path and writes it to the writer
func (p *S3Provider) Retrieve(ctx context.Context, path string, w io.Writer) error {
	if p.client == nil {
		return fmt.Errorf("s3 storage provider not initialized")
	}

	// Get object
	result, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer result.Body.Close()

	// Copy data to writer
	if _, err := io.Copy(w, result.Body); err != nil {
		return fmt.Errorf("failed to read object data: %w", err)
	}

	return nil
}

// Delete removes the file at the given path
func (p *S3Provider) Delete(ctx context.Context, path string) error {
	if p.client == nil {
		return fmt.Errorf("s3 storage provider not initialized")
	}

	// Delete object
	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// List returns a list of files matching the given prefix
func (p *S3Provider) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("s3 storage provider not initialized")
	}

	var files []FileInfo

	// List objects
	paginator := s3.NewListObjectsV2Paginator(p.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(p.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			// Get object metadata
			head, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: aws.String(p.bucket),
				Key:    obj.Key,
			})
			if err != nil {
				continue
			}

			file := FileInfo{
				Path:         aws.ToString(obj.Key),
				Size:         obj.Size,
				LastModified: aws.ToTime(obj.LastModified),
				ContentType:  aws.ToString(head.ContentType),
				IsDirectory:  strings.HasSuffix(aws.ToString(obj.Key), "/"),
				Metadata:     head.Metadata,
			}
			files = append(files, file)
		}
	}

	return files, nil
}

// GetInfo returns metadata about the file at the given path
func (p *S3Provider) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("s3 storage provider not initialized")
	}

	// Get object metadata
	head, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	info := &FileInfo{
		Path:         path,
		Size:         head.ContentLength,
		LastModified: aws.ToTime(head.LastModified),
		ContentType:  aws.ToString(head.ContentType),
		IsDirectory:  strings.HasSuffix(path, "/"),
		Metadata:     head.Metadata,
	}

	return info, nil
}

// Type returns the storage provider type
func (p *S3Provider) Type() StorageType {
	return S3
}

// Close closes any resources held by the provider
func (p *S3Provider) Close() error {
	// No resources to close for S3
	return nil
} 