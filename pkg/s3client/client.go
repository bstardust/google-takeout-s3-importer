package s3client

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config represents the configuration for an S3 client
type Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Prefix    string
}

// Client represents an S3 client
type Client struct {
	client *minio.Client
	config Config
}

// New creates a new S3 client
func New(ctx context.Context, cfg Config) (*Client, error) {
	// Validate configuration
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("S3 endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket name is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("S3 access key and secret key are required")
	}

	// Remove protocol prefix if present
	endpoint := cfg.Endpoint
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	// Initialize MinIO client
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Check if bucket exists
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check if bucket exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %s does not exist", cfg.Bucket)
	}

	logger.Info("Successfully connected to S3 endpoint %s, bucket %s", endpoint, cfg.Bucket)

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// UploadFile uploads a file to S3
func (c *Client) UploadFile(ctx context.Context, reader io.Reader, objectKey string, size int64, metadata map[string]string, contentType string) error {
	// Ensure the object key has the prefix
	objectKey = c.getObjectKey(objectKey)

	// Set default content type if not provided
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload the file
	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: metadata,
	}

	info, err := c.client.PutObject(ctx, c.config.Bucket, objectKey, reader, size, opts)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	logger.Debug("Uploaded file to %s (%d bytes, etag: %s)", objectKey, info.Size, info.ETag)
	return nil
}

// ObjectExists checks if an object exists in the bucket
func (c *Client) ObjectExists(ctx context.Context, objectKey string) (bool, error) {
	objectKey = c.getObjectKey(objectKey)

	// Try to get object info
	_, err := c.client.StatObject(ctx, c.config.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		// Check if the error is because the object doesn't exist
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if object exists: %w", err)
	}

	return true, nil
}

// ListObjects lists objects in the bucket with the given prefix
func (c *Client) ListObjects(ctx context.Context, prefix string) ([]minio.ObjectInfo, error) {
	prefix = c.getObjectKey(prefix)

	var objects []minio.ObjectInfo

	// Create a channel to receive objects
	objectCh := c.client.ListObjects(ctx, c.config.Bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	// Read objects from the channel
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}
		objects = append(objects, object)
	}

	return objects, nil
}

// GetObject retrieves an object from the bucket
func (c *Client) GetObject(ctx context.Context, objectKey string) (*minio.Object, error) {
	objectKey = c.getObjectKey(objectKey)

	// Get the object
	obj, err := c.client.GetObject(ctx, c.config.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return obj, nil
}

// DeleteObject deletes an object from the bucket
func (c *Client) DeleteObject(ctx context.Context, objectKey string) error {
	objectKey = c.getObjectKey(objectKey)

	// Delete the object
	err := c.client.RemoveObject(ctx, c.config.Bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	logger.Debug("Deleted object %s", objectKey)
	return nil
}

// GetPresignedURL generates a presigned URL for an object
func (c *Client) GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	objectKey = c.getObjectKey(objectKey)

	// Generate presigned URL
	url, err := c.client.PresignedGetObject(ctx, c.config.Bucket, objectKey, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url.String(), nil
}

// getObjectKey returns the full object key with prefix
func (c *Client) getObjectKey(key string) string {
	if c.config.Prefix == "" {
		return key
	}

	// Ensure prefix doesn't have trailing slash
	prefix := strings.TrimSuffix(c.config.Prefix, "/")

	// Ensure key doesn't have leading slash
	key = strings.TrimPrefix(key, "/")

	return filepath.Join(prefix, key)
}

// GetBucketName returns the bucket name
func (c *Client) GetBucketName() string {
	return c.config.Bucket
}

// GetEndpoint returns the endpoint
func (c *Client) GetEndpoint() string {
	return c.config.Endpoint
}

// GetPrefix returns the prefix
func (c *Client) GetPrefix() string {
	return c.config.Prefix
}
