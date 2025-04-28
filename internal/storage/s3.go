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
)

// S3Provider implements the Provider interface for AWS S3
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
	if config.AccessKey == "" {
		return fmt.Errorf("access key is required for S3 storage provider")
	}
	if config.SecretKey == "" {
		return fmt.Errorf("secret key is required for S3 storage provider")
	}

	// Create AWS credentials
	creds := credentials.NewStaticCredentialsProvider(config.AccessKey, config.SecretKey, "")

	// Create AWS config
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(creds),
		config.WithRegion(config.Region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if config.Endpoint != "" {
			o.BaseEndpoint = aws.String(config.Endpoint)
		}
	})

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

	// Convert metadata to AWS format
	awsMetadata := make(map[string]string)
	for k, v := range metadata {
		awsMetadata[k] = v
	}

	// Upload object
	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(path),
		Body:     r,
		Metadata: awsMetadata,
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
	_, err = io.Copy(w, result.Body)
	if err != nil {
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
	var continuationToken *string

	for {
		// List objects
		result, err := p.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(p.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Process objects
		for _, obj := range result.Contents {
			file := FileInfo{
				Path:         *obj.Key,
				Size:         obj.Size,
				LastModified: *obj.LastModified,
				IsDirectory:  strings.HasSuffix(*obj.Key, "/"),
			}
			files = append(files, file)
		}

		// Check if there are more objects
		if !result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}

	return files, nil
}

// GetInfo returns metadata about the file at the given path
func (p *S3Provider) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("s3 storage provider not initialized")
	}

	// Get object metadata
	result, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	info := &FileInfo{
		Path:         path,
		Size:         result.ContentLength,
		LastModified: *result.LastModified,
		ContentType:  aws.ToString(result.ContentType),
		IsDirectory:  strings.HasSuffix(path, "/"),
		Metadata:     result.Metadata,
	}

	return info, nil
}

// Type returns the storage provider type
func (p *S3Provider) Type() StorageType {
	return S3
} 