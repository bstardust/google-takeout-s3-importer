package s3client

import (
	"context"
)

// Config represents the configuration for an S3 client
type Config struct {
	Endpoint         string
	Region           string
	Bucket           string
	AccessKey        string
	SecretKey        string
	UseSSL           bool
	Prefix           string
	DisableChecksums bool
}

// Define function variables that point to the actual implementations
// These can be overridden in tests
var NewMinIOFunc = NewMinIO
var NewAWSFunc = NewAWS

// New creates a new S3 client based on configuration
func New(ctx context.Context, cfg Config) (S3Interface, error) {
	if cfg.DisableChecksums {
		// Use AWS SDK client when checksums are disabled
		return NewAWSFunc(ctx, cfg)
	}

	// Use MinIO client otherwise
	return NewMinIOFunc(ctx, cfg)
}
