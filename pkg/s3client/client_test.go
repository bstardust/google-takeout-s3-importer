package s3client

import (
	"context"
	"io"
	"net/url"
	"strings"
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
	tests := []struct {
		name      string
		config    Config
		mockSetup func(*MockMinioClient)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid configuration",
			config: Config{
				Endpoint:  "test-endpoint",
				Region:    "us-east-1",
				Bucket:    "test-bucket",
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
				UseSSL:    true,
			},
			mockSetup: func(m *MockMinioClient) {
				m.On("BucketExists", mock.Anything, "test-bucket").Return(true, nil)
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			config: Config{
				Endpoint:  "",
				Region:    "us-east-1",
				Bucket:    "test-bucket",
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
				UseSSL:    true,
			},
			mockSetup: func(m *MockMinioClient) {},
			wantErr:   true,
			errMsg:    "S3 endpoint is required",
		},
		{
			name: "missing bucket",
			config: Config{
				Endpoint:  "test-endpoint",
				Region:    "us-east-1",
				Bucket:    "",
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
				UseSSL:    true,
			},
			mockSetup: func(m *MockMinioClient) {},
			wantErr:   true,
			errMsg:    "S3 bucket name is required",
		},
		{
			name: "missing credentials",
			config: Config{
				Endpoint:  "test-endpoint",
				Region:    "us-east-1",
				Bucket:    "test-bucket",
				AccessKey: "",
				SecretKey: "test-secret-key",
				UseSSL:    true,
			},
			mockSetup: func(m *MockMinioClient) {},
			wantErr:   true,
			errMsg:    "S3 access key and secret key are required",
		},
		{
			name: "bucket does not exist",
			config: Config{
				Endpoint:  "test-endpoint",
				Region:    "us-east-1",
				Bucket:    "test-bucket",
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
				UseSSL:    true,
			},
			mockSetup: func(m *MockMinioClient) {
				m.On("BucketExists", mock.Anything, "test-bucket").Return(false, nil)
			},
			wantErr: true,
			errMsg:  "bucket test-bucket does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMinio := new(MockMinioClient)
			tt.mockSetup(mockMinio)

			// Override the minio.New function for testing
			originalMinioNew := minioNew
			defer func() { minioNew = originalMinioNew }()
			minioNew = func(endpoint string, opts *minio.Options) (*minio.Client, error) {
				return mockMinio, nil
			}

			ctx := context.Background()
			client, err := New(ctx, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestUploadFile(t *testing.T) {
	mockMinio := new(MockMinioClient)

	// Setup test data
	testData := "test file content"
	testReader := strings.NewReader(testData)
	testObjectKey := "test/file.jpg"
	testSize := int64(len(testData))
	testMetadata := map[string]string{"key": "value"}
	testContentType := "image/jpeg"

	// Configure mock
	mockMinio.On("PutObject",
		mock.Anything,
		"test-bucket",
		"prefix/test/file.jpg",
		mock.Anything,
		testSize,
		mock.MatchedBy(func(opts minio.PutObjectOptions) bool {
			return opts.ContentType == testContentType &&
				opts.UserMetadata["key"] == "value"
		}),
	).Return(minio.UploadInfo{
		Bucket:    "test-bucket",
		Key:       "prefix/test/file.jpg",
		ETag:      "test-etag",
		Size:      testSize,
		VersionID: "1",
	}, nil)

	// Create client with mock
	client := &Client{
		client: mockMinio,
		config: Config{
			Bucket: "test-bucket",
			Prefix: "prefix",
		},
	}

	// Test upload
	err := client.UploadFile(context.Background(), testReader, testObjectKey, testSize, testMetadata, testContentType)

	// Verify results
	assert.NoError(t, err)
	mockMinio.AssertExpectations(t)
}

func TestObjectExists(t *testing.T) {
	mockMinio := new(MockMinioClient)

	// Test cases
	tests := []struct {
		name       string
		objectKey  string
		mockSetup  func()
		wantExists bool
		wantErr    bool
	}{
		{
			name:      "object exists",
			objectKey: "test/existing.jpg",
			mockSetup: func() {
				mockMinio.On("StatObject",
					mock.Anything,
					"test-bucket",
					"test/existing.jpg",
					mock.Anything,
				).Return(minio.ObjectInfo{
					Key:  "test/existing.jpg",
					Size: 1024,
				}, nil)
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:      "object does not exist",
			objectKey: "test/not-found.jpg",
			mockSetup: func() {
				mockMinio.On("StatObject",
					mock.Anything,
					"test-bucket",
					"test/not-found.jpg",
					mock.Anything,
				).Return(minio.ObjectInfo{}, minio.ErrorResponse{
					Code: "NoSuchKey",
				})
			},
			wantExists: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock and setup for this test case
			mockMinio = new(MockMinioClient)
			tt.mockSetup()

			// Create client with mock
			client := &Client{
				client: mockMinio,
				config: Config{
					Bucket: "test-bucket",
				},
			}

			// Test exists check
			exists, err := client.ObjectExists(context.Background(), tt.objectKey)

			// Verify results
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantExists, exists)
			}
			mockMinio.AssertExpectations(t)
		})
	}
}
