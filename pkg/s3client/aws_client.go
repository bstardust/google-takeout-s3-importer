package s3client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/minio/minio-go/v7"
)

// AWSClient represents an S3 client using AWS SDK v1.72.3
type AWSClient struct {
	client   *s3.S3
	uploader *s3manager.Uploader
	config   Config
}

// NewAWS creates a new AWS S3 client
func NewAWS(ctx context.Context, cfg Config) (S3Interface, error) {
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

	// Ensure endpoint has proper format
	endpoint := cfg.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if cfg.UseSSL {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}

	// Initialize AWS session
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(cfg.Region),
		S3ForcePathStyle: aws.Bool(true),
		DisableSSL:       aws.Bool(!cfg.UseSSL),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create S3 client
	client := s3.New(newSession)

	// Validate bucket exists
	_, err = client.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check if bucket exists: %w", err)
	}

	logger.Info("Successfully connected to S3 endpoint %s, bucket %s using AWS SDK", endpoint, cfg.Bucket)

	// Create S3 client with custom part size configuration
	uploader := s3manager.NewUploaderWithClient(client, func(u *s3manager.Uploader) {
		// Set minimum part size to 5MB (B2 requirement)
		u.PartSize = 5 * 1024 * 1024
		// Set concurrency to match our app's concurrency
		u.Concurrency = 4
		// Disable automatic content-type detection which can cause issues
		u.LeavePartsOnError = false
	})

	return &AWSClient{
		client:   client,
		uploader: uploader,
		config:   cfg,
	}, nil
}

// UploadFile uploads a file to S3
func (c *AWSClient) UploadFile(ctx context.Context, reader io.Reader, objectKey string, size int64, metadata map[string]string, contentType string) error {
	// Ensure the object key has the prefix
	objectKey = c.getObjectKey(objectKey)

	// Set default content type if not provided
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Convert metadata map to AWS format
	awsMetadata := make(map[string]*string)
	for k, v := range metadata {
		value := v
		awsMetadata[k] = &value
	}

	// For small files (less than 10MB), use PutObject instead of multipart upload
	// to avoid the "request body too small" error with B2
	if size < 10*1024*1024 {
		// Buffer the file in memory - safe for small files
		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, reader); err != nil {
			return fmt.Errorf("failed to buffer file: %w", err)
		}

		_, err := c.client.PutObjectWithContext(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(c.config.Bucket),
			Key:         aws.String(objectKey),
			Body:        bytes.NewReader(buf.Bytes()),
			ContentType: aws.String(contentType),
			Metadata:    awsMetadata,
		})

		if err != nil {
			return fmt.Errorf("failed to upload file: %w", err)
		}
	} else {
		// For larger files, use multipart upload with adjusted settings
		uploader := s3manager.NewUploaderWithClient(c.client, func(u *s3manager.Uploader) {
			// Backblaze B2 requires at least 5MB parts
			u.PartSize = 10 * 1024 * 1024 // Use 10MB to be safe
			u.Concurrency = 4
			u.LeavePartsOnError = false
		})

		_, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
			Bucket:      aws.String(c.config.Bucket),
			Key:         aws.String(objectKey),
			Body:        reader,
			ContentType: aws.String(contentType),
			Metadata:    awsMetadata,
		})

		if err != nil {
			return fmt.Errorf("failed to upload file: %w", err)
		}
	}

	logger.Debug("Uploaded file to %s (%d bytes)", objectKey, size)
	return nil
}

// ObjectExists checks if an object exists in the bucket
func (c *AWSClient) ObjectExists(ctx context.Context, objectKey string) (bool, error) {
	objectKey = c.getObjectKey(objectKey)

	_, err := c.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.config.Bucket),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if object exists: %w", err)
	}

	return true, nil
}

// ListObjects lists objects in the bucket with the given prefix
func (c *AWSClient) ListObjects(ctx context.Context, prefix string) ([]minio.ObjectInfo, error) {
	prefix = c.getObjectKey(prefix)

	var objects []minio.ObjectInfo
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            aws.String(c.config.Bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		}

		result, err := c.client.ListObjectsV2WithContext(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("error listing objects: %w", err)
		}

		// Convert AWS objects to MinIO objects for compatibility
		for _, item := range result.Contents {
			objects = append(objects, minio.ObjectInfo{
				Key:          *item.Key,
				Size:         *item.Size,
				LastModified: *item.LastModified,
				ETag:         *item.ETag,
			})
		}

		if !*result.IsTruncated {
			break
		}

		continuationToken = result.NextContinuationToken
	}

	return objects, nil
}

// GetObject retrieves an object from the bucket
// Note: This returns a MinIO Object for interface compatibility
// but internally uses AWS SDK to get the data
func (c *AWSClient) GetObject(ctx context.Context, objectKey string) (*minio.Object, error) {
	// This method requires returning a MinIO object
	// For compatibility, we'll convert AWS SDK response into a MinIO object
	// This is not ideal but necessary for interface compatibility
	return nil, fmt.Errorf("GetObject not implemented for AWS SDK client - use direct AWS SDK operations instead")
}

// DeleteObject deletes an object from the bucket
func (c *AWSClient) DeleteObject(ctx context.Context, objectKey string) error {
	objectKey = c.getObjectKey(objectKey)

	_, err := c.client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.config.Bucket),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	logger.Debug("Deleted object %s", objectKey)
	return nil
}

// GetPresignedURL generates a presigned URL for an object
func (c *AWSClient) GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	objectKey = c.getObjectKey(objectKey)

	req, _ := c.client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(c.config.Bucket),
		Key:    aws.String(objectKey),
	})

	urlStr, err := req.Presign(expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return urlStr, nil
}

// getObjectKey returns the full object key with prefix
func (c *AWSClient) getObjectKey(key string) string {
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
func (c *AWSClient) GetBucketName() string {
	return c.config.Bucket
}

// GetEndpoint returns the endpoint
func (c *AWSClient) GetEndpoint() string {
	return c.config.Endpoint
}

// GetPrefix returns the prefix
func (c *AWSClient) GetPrefix() string {
	return c.config.Prefix
}
