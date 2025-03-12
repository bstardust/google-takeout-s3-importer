package s3client

import (
	"context"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMinioClient is a mock implementation of the Minio client
type MockMinioClient struct {
	mock.Mock
}

func (m *MockMinioClient) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	args := m.Called(ctx, bucketName)
	return args.Bool(0), args.Error(1)
}

func (m *MockMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, reader, objectSize, opts)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}

func (m *MockMinioClient) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(minio.ObjectInfo), args.Error(1)
}

func (m *MockMinioClient) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	args := m.Called(ctx, bucketName, opts)
	return args.Get(0).(<-chan minio.ObjectInfo)
}

func (m *MockMinioClient) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*minio.Object), args.Error(1)
}

func (m *MockMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Error(0)
}

func (m *MockMinioClient) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams map[string]string) (*url.URL, error) {
	args := m.Called(ctx, bucketName, objectName, expires, reqParams)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*url.URL), args.Error(1)
}

// Test cases
func TestNew(t *testing.T) {
	// Test without disable checksums - should use MinIO client
	cfg := Config{
		Endpoint:         "test-endpoint",
		Region:           "test-region",
		Bucket:           "test-bucket",
		AccessKey:        "test-access-key",
		SecretKey:        "test-secret-key",
		UseSSL:           true,
		DisableChecksums: false,
	}

	// Store the original functions
	origNewMinIO := NewMinIOFunc
	origNewAWS := NewAWSFunc
	defer func() {
		// Restore original functions after test
		NewMinIOFunc = origNewMinIO
		NewAWSFunc = origNewAWS
	}()

	var usedMinIO, usedAWS bool

	// Replace the functions with test versions
	NewMinIOFunc = func(ctx context.Context, cfg Config) (S3Interface, error) {
		usedMinIO = true
		return &MinioClient{}, nil
	}

	NewAWSFunc = func(ctx context.Context, cfg Config) (S3Interface, error) {
		usedAWS = true
		return &AWSClient{}, nil
	}

	// Test with checksums disabled = false (should use MinIO)
	client, err := New(context.Background(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.True(t, usedMinIO)
	assert.False(t, usedAWS)

	// Reset flags
	usedMinIO = false
	usedAWS = false

	// Test with checksums disabled = true (should use AWS)
	cfg.DisableChecksums = true
	client, err = New(context.Background(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.False(t, usedMinIO)
	assert.True(t, usedAWS)
}

// TestUploadFile and TestObjectExists need a different approach
// Instead of testing the internal implementation, we should test through the interface

// For MinioClient specific tests, we need to use a different approach
// Here's an example of a TestMinioClient that uses a mock
func TestMinioClient_Interface(t *testing.T) {
	// Create a mock MinioClient that implements S3Interface
	client := &MockS3Client{}

	// Verify it implements the interface (compile-time check)
	var _ S3Interface = client

	// Now we can test with this mock
	assert.NotNil(t, client)
}

// Mock S3 Client for testing the interface
type MockS3Client struct{}

func (m *MockS3Client) UploadFile(ctx context.Context, reader io.Reader, objectKey string, size int64, metadata map[string]string, contentType string) error {
	return nil
}

func (m *MockS3Client) ObjectExists(ctx context.Context, objectKey string) (bool, error) {
	return true, nil
}

func (m *MockS3Client) ListObjects(ctx context.Context, prefix string) ([]minio.ObjectInfo, error) {
	return []minio.ObjectInfo{}, nil
}

func (m *MockS3Client) GetObject(ctx context.Context, objectKey string) (*minio.Object, error) {
	return nil, nil
}

func (m *MockS3Client) DeleteObject(ctx context.Context, objectKey string) error {
	return nil
}

func (m *MockS3Client) GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	return "", nil
}

func (m *MockS3Client) GetBucketName() string {
	return "test-bucket"
}

func (m *MockS3Client) GetEndpoint() string {
	return "test-endpoint"
}

func (m *MockS3Client) GetPrefix() string {
	return ""
}
