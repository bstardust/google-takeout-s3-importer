package uploader

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/bstardust/google-takeout-s3-importer/internal/adapter/googletakeout"
	"github.com/bstardust/google-takeout-s3-importer/internal/config"
	"github.com/bstardust/google-takeout-s3-importer/internal/journal"
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/bstardust/google-takeout-s3-importer/internal/progress"
	"github.com/bstardust/google-takeout-s3-importer/internal/worker"
	"github.com/bstardust/google-takeout-s3-importer/pkg/s3client"
)

// Uploader handles the upload process
type Uploader struct {
	ctx      context.Context
	s3Client *s3client.Client
	takeout  *googletakeout.Takeout
	journal  *journal.Journal
	pool     *worker.Pool
	progress *progress.Reporter
	config   *config.Config
}

// New creates a new Uploader
func New(ctx context.Context, s3Client *s3client.Client, takeout *googletakeout.Takeout,
	jnl *journal.Journal, pool *worker.Pool, progress *progress.Reporter,
	cfg *config.Config) *Uploader {
	return &Uploader{
		ctx:      ctx,
		s3Client: s3Client,
		takeout:  takeout,
		journal:  jnl,
		pool:     pool,
		progress: progress,
		config:   cfg,
	}
}

// Run starts the upload process
func (u *Uploader) Run() error {
	files := u.takeout.ListFiles()

	// Initialize progress reporter
	u.progress.Start(len(files))

	// Create a wait group to wait for all uploads to complete
	var wg sync.WaitGroup

	// Process each file
	for _, file := range files {
		// Check if the context has been canceled
		if u.ctx.Err() != nil {
			return u.ctx.Err()
		}

		// Skip if already uploaded
		if u.journal.IsUploaded(file.Path) && u.config.Upload.SkipExisting {
			u.progress.Skip(file.Path)
			continue
		}

		// Check if the file already exists in S3
		if u.config.Upload.SkipExisting {
			exists, err := u.s3Client.ObjectExists(u.ctx, file.Path)
			if err != nil {
				logger.Warn("Failed to check if file exists: %v", err)
			} else if exists {
				u.progress.Skip(file.Path)
				u.journal.MarkUploaded(file.Path)
				continue
			}
		}

		// Add the file to the upload queue
		wg.Add(1)
		u.pool.Submit(func() {
			defer wg.Done()

			if err := u.uploadFile(file); err != nil {
				logger.Error("Failed to upload %s: %v", file.Path, err)
				u.progress.Error(file.Path, err)
			} else {
				u.progress.Complete(file.Path)
				u.journal.MarkUploaded(file.Path)
				u.journal.Save() // Save after each successful upload
			}
		})
	}

	// Wait for all uploads to complete
	wg.Wait()

	// Finalize progress
	u.progress.Finish()

	return nil
}

// uploadFile uploads a single file (updated function)
func (u *Uploader) uploadFile(file *googletakeout.MediaFile) error {
	// Open the file
	reader, err := u.takeout.OpenFile(file.Path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer reader.Close()

	// Get file size
	size := file.Size
	if size <= 0 {
		// If size is not available, try to get it from the reader
		if seeker, ok := reader.(io.Seeker); ok {
			size, err = seeker.Seek(0, io.SeekEnd)
			if err != nil {
				return fmt.Errorf("failed to get file size: %w", err)
			}

			// Reset reader to beginning
			_, err = seeker.Seek(0, io.SeekStart)
			if err != nil {
				return fmt.Errorf("failed to reset reader: %w", err)
			}
		} else {
			return fmt.Errorf("unable to determine file size for %s", file.Path)
		}
	}

	// Prepare metadata
	var metadataMap map[string]string

	if u.config.Upload.PreserveMetadata && file.Metadata != nil {
		// Convert metadata to map
		metadataMap = file.Metadata.ToMap()
	} else {
		metadataMap = make(map[string]string)
	}

	// Add original filename as metadata
	metadataMap["original-filename"] = filepath.Base(file.Path)

	// Detect content type
	contentType := s3client.DetectContentType(file.Path)

	// Perform the upload (or simulate in dry-run mode)
	if u.config.Upload.DryRun {
		logger.Info("DRY RUN: Would upload %s (%d bytes, %s) with %d metadata fields",
			file.Path, size, contentType, len(metadataMap))
		return nil
	}

	return u.s3Client.UploadFile(u.ctx, reader, file.Path, size, metadataMap, contentType)
}
