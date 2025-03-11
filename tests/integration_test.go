package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/config"
	"github.com/bstardust/google-takeout-s3-importer/internal/journal"
	"github.com/bstardust/google-takeout-s3-importer/internal/progress"
	"github.com/bstardust/google-takeout-s3-importer/internal/worker"
	"github.com/bstardust/google-takeout-s3-importer/pkg/s3client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests require a running S3-compatible server
// You can use MinIO in Docker for local testing:
// docker run -p 9000:9000 -p 9001:9001 minio/minio server /data --console-address ":9001"

func TestIntegrationUpload(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run")
	}

	// Test configuration
	cfg := &config.Config{
		S3: config.S3Config{
			Endpoint:  getEnvOrDefault("TEST_S3_ENDPOINT", "localhost:9000"),
			Region:    getEnvOrDefault("TEST_S3_REGION", "us-east-1"),
			Bucket:    getEnvOrDefault("TEST_S3_BUCKET", "test-bucket"),
			AccessKey: getEnvOrDefault("TEST_S3_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnvOrDefault("TEST_S3_SECRET_KEY", "minioadmin"),
			UseSSL:    os.Getenv("TEST_S3_USE_SSL") == "true",
			Prefix:    "integration-test",
		},
		Upload: config.UploadConfig{
			Concurrency:      2,
			DryRun:           false,
			Resume:           true,
			PreserveMetadata: true,
			SkipExisting:     true,
		},
	}

	// Create test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Initialize S3 client
	s3Config := s3client.Config{
		Endpoint:  cfg.S3.Endpoint,
		Region:    cfg.S3.Region,
		Bucket:    cfg.S3.Bucket,
		AccessKey: cfg.S3.AccessKey,
		SecretKey: cfg.S3.SecretKey,
		UseSSL:    cfg.S3.UseSSL,
		Prefix:    cfg.S3.Prefix,
	}

	// Create S3 client
	s3Client, err := s3client.New(ctx, s3Config)
	require.NoError(t, err, "Failed to create S3 client")

	// Test cleanup - delete test prefix after test
	defer func() {
		// List objects in test prefix
		objects, err := s3Client.ListObjects(context.Background(), "")
		if err == nil {
			// Delete each object
			for _, obj := range objects {
				s3Client.DeleteObject(context.Background(), obj.Key)
			}
		}
	}()

	// Test takeout directory path
	takeoutPath := getEnvOrDefault("TEST_TAKEOUT_PATH", "./testdata/takeout")
	require.DirExists(t, takeoutPath, "Test takeout directory does not exist")

	// Create test components
	jnl := journal.New("test-journal.json")
	pool := worker.NewPool(cfg.Upload.Concurrency)
	progressReporter := progress.New()

	// Create takeout adapter
	takeout, err := googletakeout.New(ctx, takeoutPath, false)
	require.NoError(t, err, "Failed to create Google Takeout adapter")

	// Create uploader
	uploader := uploader.New(ctx, s3Client, takeout, jnl, pool, progressReporter, cfg)

	// Run the upload process
	err = uploader.Run()
	assert.NoError(t, err, "Upload process failed")

	// Verify files were uploaded
	files := takeout.ListFiles()
	assert.Greater(t, len(files), 0, "No files found in takeout")

	// Check a few random files exist in S3
	for i := 0; i < min(3, len(files)); i++ {
		exists, err := s3Client.ObjectExists(ctx, files[i].Path)
		assert.NoError(t, err, "Failed to check if object exists")
		assert.True(t, exists, "Uploaded file does not exist in S3")
	}

	// Check journal has all files marked as completed
	completed := jnl.ListCompleted()
	assert.Equal(t, len(files), len(completed), "Not all files were marked as completed in journal")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
