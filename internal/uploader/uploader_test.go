package uploader

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/adapter/googletakeout"
	"github.com/bstardust/google-takeout-s3-importer/internal/config"
	"github.com/bstardust/google-takeout-s3-importer/internal/journal"
	"github.com/bstardust/google-takeout-s3-importer/internal/metadata"
	"github.com/bstardust/google-takeout-s3-importer/internal/progress"
	"github.com/bstardust/google-takeout-s3-importer/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock S3 Client
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) UploadFile(ctx context.Context, reader io.Reader, objectKey string, size int64, metadata map[string]string, contentType string) error {
	args := m.Called(ctx, reader, objectKey, size, metadata, contentType)
	return args.Error(0)
}

func (m *MockS3Client) ObjectExists(ctx context.Context, objectKey string) (bool, error) {
	args := m.Called(ctx, objectKey)
	return args.Bool(0), args.Error(1)
}

func (m *MockS3Client) GetBucketName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockS3Client) GetEndpoint() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockS3Client) GetPrefix() string {
	args := m.Called()
	return args.String(0)
}

// Mock Google Takeout
type MockTakeout struct {
	mock.Mock
}

func (m *MockTakeout) ListFiles() []*googletakeout.MediaFile {
	args := m.Called()
	return args.Get(0).([]*googletakeout.MediaFile)
}

func (m *MockTakeout) OpenFile(path string) (io.ReadCloser, error) {
	args := m.Called(path)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockTakeout) GetMetadata(path string) *metadata.Metadata {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*metadata.Metadata)
}

func (m *MockTakeout) GetSize(path string) int64 {
	args := m.Called(path)
	return args.Get(0).(int64)
}

// Mock ReadCloser
type MockReadCloser struct {
	io.Reader
}

func (m MockReadCloser) Close() error {
	return nil
}

// Tests
func TestUploader_Run(t *testing.T) {
	// Create mocks
	mockS3 := new(MockS3Client)
	mockTakeout := new(MockTakeout)

	// Create test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create test config
	cfg := &config.Config{
		Upload: config.UploadConfig{
			SkipExisting:     true,
			PreserveMetadata: true,
			DryRun:           false,
		},
	}

	// Create components
	jnl := journal.New("")
	pool := worker.NewPool(2)
	prog := progress.New()

	// Setup test media files
	mediaFiles := []*googletakeout.MediaFile{
		{
			Path: "test/photo1.jpg",
			Metadata: &metadata.Metadata{
				CreationTime: time.Now(),
				MimeType:     "image/jpeg",
				Title:        "Photo 1",
			},
			Size: 1024,
		},
		{
			Path: "test/photo2.jpg",
			Metadata: &metadata.Metadata{
				CreationTime: time.Now(),
				MimeType:     "image/jpeg",
				Title:        "Photo 2",
			},
			Size: 2048,
		},
	}

	// Configure mock expectations
	mockTakeout.On("ListFiles").Return(mediaFiles)

	// First file doesn't exist in S3
	mockS3.On("ObjectExists", ctx, "test/photo1.jpg").Return(false, nil)
	mockTakeout.On("GetSize", "test/photo1.jpg").Return(int64(1024))
	mockTakeout.On("GetMetadata", "test/photo1.jpg").Return(mediaFiles[0].Metadata)
	mockTakeout.On("OpenFile", "test/photo1.jpg").Return(
		MockReadCloser{Reader: strings.NewReader("test file content")},
		nil,
	)
	mockS3.On("UploadFile", ctx, mock.Anything, "test/photo1.jpg", int64(1024), mock.Anything, "image/jpeg").Return(nil)

	// Second file already exists in S3
	mockS3.On("ObjectExists", ctx, "test/photo2.jpg").Return(true, nil)

	// Mock bucket info
	mockS3.On("GetBucketName").Return("test-bucket")
	mockS3.On("GetEndpoint").Return("test-endpoint")
	mockS3.On("GetPrefix").Return("")

	// Create uploader with mocks
	uploader := New(ctx, mockS3, mockTakeout, jnl, pool, prog, cfg)

	// Run the uploader
	err := uploader.Run()

	// Verify results
	assert.NoError(t, err)
	mockS3.AssertExpectations(t)
	mockTakeout.AssertExpectations(t)

	// Check journal has the completed file
	completed := jnl.ListCompleted()
	assert.Contains(t, completed, "test/photo1.jpg")
}

func TestUploader_Run_WithError(t *testing.T) {
	// Create mocks
	mockS3 := new(MockS3Client)
	mockTakeout := new(MockTakeout)

	// Create test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create test config
	cfg := &config.Config{
		Upload: config.UploadConfig{
			SkipExisting:     true,
			PreserveMetadata: true,
			DryRun:           false,
		},
	}

	// Create components
	jnl := journal.New("")
	pool := worker.NewPool(2)
	prog := progress.New()

	// Setup test media file
	mediaFiles := []*googletakeout.MediaFile{
		{
			Path: "test/photo_error.jpg",
			Metadata: &metadata.Metadata{
				CreationTime: time.Now(),
				MimeType:     "image/jpeg",
				Title:        "Photo Error",
			},
			Size: 1024,
		},
	}

	// Configure mock expectations
	mockTakeout.On("ListFiles").Return(mediaFiles)
	mockS3.On("ObjectExists", ctx, "test/photo_error.jpg").Return(false, nil)
	mockTakeout.On("GetSize", "test/photo_error.jpg").Return(int64(1024))
	mockTakeout.On("GetMetadata", "test/photo_error.jpg").Return(mediaFiles[0].Metadata)
	mockTakeout.On("OpenFile", "test/photo_error.jpg").Return(
		MockReadCloser{Reader: strings.NewReader("test file content")},
		nil,
	)

	// Simulate upload error
	uploadErr := errors.New("upload failed: network error")
	mockS3.On("UploadFile", ctx, mock.Anything, "test/photo_error.jpg", int64(1024), mock.Anything, "image/jpeg").Return(uploadErr)

	// Mock bucket info
	mockS3.On("GetBucketName").Return("test-bucket")
	mockS3.On("GetEndpoint").Return("test-endpoint")
	mockS3.On("GetPrefix").Return("")

	// Create uploader with mocks
	uploader := New(ctx, mockS3, mockTakeout, jnl, pool, prog, cfg)

	// Run the uploader
	err := uploader.Run()

	// Verify error is returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upload failed")

	// Check journal doesn't have the failed file
	completed := jnl.ListCompleted()
	assert.NotContains(t, completed, "test/photo_error.jpg")
}
