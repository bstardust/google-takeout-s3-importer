// internal/adapter/s3/client.go
package s3

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/bstardust/google-takeout-s3-importer/internal/config"
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client represents an S3 client
type Client struct {
	client *minio.Client
	config config.S3Config
}

// NewClient creates a new S3 client
func NewClient(ctx context.Context, cfg config.S3Config) (*Client, error) {
	// Initialize MinIO client
	client, err := minio.New(cfg.Endpoint, &minio.Options{
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

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// UploadFile uploads a file to S3
func (c *Client) UploadFile(ctx context.Context, reader io.Reader, objectKey string, size int64, metadata map[string]string) error {
	// Ensure the object key has the prefix
	objectKey = c.getObjectKey(objectKey)

	// Upload the file
	_, err := c.client.PutObject(ctx, c.config.Bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType:  "application/octet-stream",
		UserMetadata: metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	logger.Debug("Uploaded file to %s", objectKey)
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

// getObjectKey returns the full object key with prefix
func (c *Client) getObjectKey(key string) string {
	if c.config.Prefix == "" {
		return key
	}
	return filepath.Join(c.config.Prefix, key)
}
