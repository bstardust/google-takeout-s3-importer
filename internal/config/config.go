package config

import (
	"time"
)

// Config represents the application configuration
type Config struct {
	LogLevel string
	S3       S3Config
	Upload   UploadConfig
}

// S3Config represents S3 connection configuration
type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Prefix    string
}

// UploadConfig represents upload configuration
type UploadConfig struct {
	Concurrency      int
	DryRun           bool
	Resume           bool
	JournalPath      string
	PreserveMetadata bool
	SkipExisting     bool
	Timeout          time.Duration
}

// New creates a new configuration with default values
func New() *Config {
	return &Config{
		LogLevel: "info",
		S3: S3Config{
			Region: "us-east-1",
			UseSSL: true,
		},
		Upload: UploadConfig{
			Concurrency:      4,
			Resume:           true,
			PreserveMetadata: true,
			SkipExisting:     true,
			Timeout:          30 * time.Minute,
		},
	}
}
