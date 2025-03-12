package s3client

import (
	"context"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

// S3Interface defines the operations that an S3 client must implement
type S3Interface interface {
	UploadFile(ctx context.Context, reader io.Reader, objectKey string, size int64, metadata map[string]string, contentType string) error
	ObjectExists(ctx context.Context, objectKey string) (bool, error)
	ListObjects(ctx context.Context, prefix string) ([]minio.ObjectInfo, error)
	GetObject(ctx context.Context, objectKey string) (*minio.Object, error)
	DeleteObject(ctx context.Context, objectKey string) error
	GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error)
	GetBucketName() string
	GetEndpoint() string
	GetPrefix() string
}
