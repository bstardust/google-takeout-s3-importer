package uploader

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/adapter/googletakeout"
	"github.com/bstardust/google-takeout-s3-importer/internal/config"
	"github.com/bstardust/google-takeout-s3-importer/internal/journal"
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/bstardust/google-takeout-s3-importer/internal/progress"
	"github.com/bstardust/google-takeout-s3-importer/internal/worker"
	"github.com/bstardust/google-takeout-s3-importer/pkg/s3client"
)

// Uploader handles the process of uploading files from Google Takeout to S3
type Uploader struct {
	ctx      context.Context
	s3Client *s3client.Client
	takeout  *googletakeout.Takeout
	journal  *journal.Journal
	pool     *worker.Pool
	progress *progress.Reporter
	config   *config.Config

	// Statistics
	totalFiles    int
	uploadedFiles int32
	skippedFiles  int32
	failedFiles   int32
	totalBytes    int64
	uploadedBytes int64

	// Error handling
	retryConfig RetryConfig
}

// New creates a new Uploader
func New(ctx context.Context, s3Client *s3client.Client, takeout *googletakeout.Takeout,
	jnl *journal.Journal, pool *worker.Pool, progress *progress.Reporter,
	cfg *config.Config) *Uploader {

	return &Uploader{
		ctx:         ctx,
		s3Client:    s3Client,
		takeout:     takeout,
		journal:     jnl,
		pool:        pool,
		progress:    progress,
		config:      cfg,
		retryConfig: DefaultRetryConfig(),
	}
}

// Run executes the upload process
func (u *Uploader) Run() error {
	// Get files to process
	files := u.takeout.ListFiles()
	u.totalFiles = len(files)

	if u.totalFiles == 0 {
		logger.Warn("No files found in the provided Google Takeout archive")
		return nil
	}

	// Calculate total size
	for _, file := range files {
		u.totalBytes += file.Size
	}

	logger.Info("Starting upload to %s bucket %s", u.s3Client.GetEndpoint(), u.s3Client.GetBucketName())
	logger.Info("Found %d files to process (%.2f MB total)", u.totalFiles, float64(u.totalBytes)/(1024*1024))

	// Start progress reporting
	if u.progress != nil {
		u.progress.Start(u.totalFiles)
		defer u.progress.Finish()
	}

	// Process each file
	var errCount int32

	// Create a channel for collecting errors
	errCh := make(chan error, u.totalFiles)

	// Submit upload tasks to the worker pool
	for _, file := range files {
		// Skip if already uploaded in journal
		if u.journal != nil && u.journal.IsUploaded(file.Path) {
			logger.Debug("Skipping already uploaded file: %s", file.Path)
			atomic.AddInt32(&u.skippedFiles, 1)
			if u.progress != nil {
				u.progress.Skip(file.Path)
			}
			continue
		}

		// Create a context for this specific file with timeout
		fileCtx, cancel := context.WithTimeout(u.ctx, 30*time.Minute)

		// Capture the file for closure
		mediaFile := file

		// Submit the task to the worker pool
		u.pool.Submit(func() {
			defer cancel()

			// Upload the file
			if err := u.uploadFile(fileCtx, mediaFile); err != nil {
				logger.Error("Failed to upload %s: %v", mediaFile.Path, err)
				atomic.AddInt32(&u.failedFiles, 1)
				if u.progress != nil {
					u.progress.Error(mediaFile.Path, err)
				}
				errCh <- fmt.Errorf("failed to upload %s: %w", mediaFile.Path, err)
				atomic.AddInt32(&errCount, 1)
			}
		})
	}

	// Wait for all tasks to complete
	u.pool.Wait()
	close(errCh)

	// Handle errors
	var err error
	if errCount > 0 {
		// Collect all errors
		var errMsgs []string
		for e := range errCh {
			errMsgs = append(errMsgs, e.Error())
			if len(errMsgs) >= 10 {
				errMsgs = append(errMsgs, fmt.Sprintf("... and %d more errors", errCount-10))
				break
			}
		}

		err = fmt.Errorf("upload completed with %d/%d files failed:\n%s",
			errCount, u.totalFiles, strings.Join(errMsgs, "\n"))
	}

	// Log summary
	u.logSummary()

	return err
}

// uploadFile handles uploading a single file to S3
func (u *Uploader) uploadFile(ctx context.Context, file *googletakeout.MediaFile) error {
	filePath := file.Path

	// Check if the file already exists in S3
	if u.config.Upload.SkipExisting {
		operation := fmt.Sprintf("Check existence of %s", filePath)

		var exists bool
		checkErr := RetryWithBackoff(ctx, operation, func() error {
			var err error
			exists, err = u.s3Client.ObjectExists(ctx, filePath)
			return err
		}, u.retryConfig)

		if checkErr != nil {
			return fmt.Errorf("failed to check if file exists: %w", checkErr)
		}

		if exists {
			logger.Debug("File already exists in S3, skipping: %s", filePath)
			atomic.AddInt32(&u.skippedFiles, 1)
			if u.progress != nil {
				u.progress.Skip(filePath)
			}
			return nil
		}
	}

	// Dry run mode
	if u.config.Upload.DryRun {
		logger.Info("[DRY RUN] Would upload %s (%.2f MB)", filePath, float64(file.Size)/(1024*1024))
		atomic.AddInt32(&u.uploadedFiles, 1)
		atomic.AddInt64(&u.uploadedBytes, file.Size)
		if u.progress != nil {
			u.progress.Complete(filePath)
		}
		if u.journal != nil {
			u.journal.MarkUploaded(filePath)
		}
		return nil
	}

	// Get file metadata
	metadata := make(map[string]string)
	if u.config.Upload.PreserveMetadata {
		if fileMetadata := u.takeout.GetMetadata(filePath); fileMetadata != nil {
			// Instead of manually constructing metadata, use the ToMap method
			metadata = fileMetadata.ToMap()

			// Add source info if not already present
			if _, ok := metadata["Source"]; !ok {
				metadata["Source"] = "Google Takeout"
			}
		}
	}

	// Determine content type
	contentType := "application/octet-stream"

	// Try to determine content type from extension first
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".mp4":
		contentType = "video/mp4"
	case ".mov":
		contentType = "video/quicktime"
	case ".heic":
		contentType = "image/heic"
	case ".3gp":
		contentType = "video/3gpp"
	case ".webp":
		contentType = "image/webp"
	}

	// If available, get content type from metadata (it might be stored in a different place)
	if file.Metadata != nil {
		// Check if we can find content type info in the metadata map
		metadataMap := file.Metadata.ToMap()
		if contentTypeFromMeta, ok := metadataMap["Content-Type"]; ok && contentTypeFromMeta != "" {
			contentType = contentTypeFromMeta
		}
	}

	// Open the file
	operation := fmt.Sprintf("Open file %s", filePath)
	var reader io.ReadCloser
	openErr := RetryWithBackoff(ctx, operation, func() error {
		var err error
		reader, err = u.takeout.OpenFile(filePath)
		return err
	}, u.retryConfig)

	if openErr != nil {
		return fmt.Errorf("failed to open file: %w", openErr)
	}
	defer reader.Close()

	// Upload the file with retry
	uploadOperation := fmt.Sprintf("Upload %s to S3", filePath)
	uploadErr := RetryWithBackoff(ctx, uploadOperation, func() error {
		return u.s3Client.UploadFile(ctx, reader, filePath, file.Size, metadata, contentType)
	}, u.retryConfig)

	if uploadErr != nil {
		return fmt.Errorf("failed to upload file: %w", uploadErr)
	}

	// Update statistics
	atomic.AddInt32(&u.uploadedFiles, 1)
	atomic.AddInt64(&u.uploadedBytes, file.Size)

	// Update progress
	if u.progress != nil {
		u.progress.Complete(filePath)
	}

	// Mark as uploaded in journal
	if u.journal != nil {
		u.journal.MarkUploaded(filePath)
	}

	logger.Debug("Successfully uploaded %s (%.2f MB)", filePath, float64(file.Size)/(1024*1024))
	return nil
}

// logSummary logs a summary of the upload process
func (u *Uploader) logSummary() {
	uploadedFiles := atomic.LoadInt32(&u.uploadedFiles)
	skippedFiles := atomic.LoadInt32(&u.skippedFiles)
	failedFiles := atomic.LoadInt32(&u.failedFiles)

	logger.Info("Upload complete:")
	logger.Info("  Total files: %d", u.totalFiles)
	logger.Info("  Uploaded: %d (%.2f MB)", uploadedFiles, float64(u.uploadedBytes)/(1024*1024))
	logger.Info("  Skipped: %d", skippedFiles)
	logger.Info("  Failed: %d", failedFiles)

	if u.config.Upload.DryRun {
		logger.Info("Note: This was a dry run, no files were actually uploaded")
	}
}
