package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bstardust/google-takeout-s3-importer/internal/adapter/googletakeout"
	"github.com/bstardust/google-takeout-s3-importer/internal/config"
	"github.com/bstardust/google-takeout-s3-importer/internal/journal"
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/bstardust/google-takeout-s3-importer/internal/progress"
	"github.com/bstardust/google-takeout-s3-importer/internal/uploader"
	"github.com/bstardust/google-takeout-s3-importer/internal/worker"
	"github.com/bstardust/google-takeout-s3-importer/pkg/s3client"
	"github.com/spf13/cobra"
)

func newUploadCommand(ctx context.Context, cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload [flags] <takeout-*.zip> | <takeout-folder>",
		Short: "Upload Google Takeout archives to S3",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			isGlob, _ := cmd.Flags().GetBool("glob")
			return runUpload(cmd.Context(), cfg, args, isGlob)
		},
	}

	// S3 connection flags
	cmd.Flags().StringVar(&cfg.S3.Endpoint, "endpoint", "", "S3 endpoint URL (required)")
	cmd.Flags().StringVar(&cfg.S3.Region, "region", "us-east-1", "S3 region")
	cmd.Flags().StringVar(&cfg.S3.Bucket, "bucket", "", "S3 bucket name (required)")
	cmd.Flags().StringVar(&cfg.S3.AccessKey, "access-key", "", "S3 access key (required)")
	cmd.Flags().StringVar(&cfg.S3.SecretKey, "secret-key", "", "S3 secret key (required)")
	cmd.Flags().BoolVar(&cfg.S3.UseSSL, "use-ssl", true, "Use SSL for S3 connection")
	cmd.Flags().StringVar(&cfg.S3.Prefix, "prefix", "", "Prefix for S3 object keys")
	cmd.Flags().BoolVar(&cfg.S3.DisableChecksums, "disable-checksums", false, "Disable checksum headers for better compatibility with Backblaze B2 (uses AWS SDK)")

	// Upload options
	cmd.Flags().IntVar(&cfg.Upload.Concurrency, "concurrency", 4, "Number of concurrent file uploads within each archive")
	cmd.Flags().IntVar(&cfg.Upload.MaxConcurrentArchives, "max-archives", 3, "Maximum number of archives to process simultaneously")
	cmd.Flags().BoolVar(&cfg.Upload.DryRun, "dry-run", false, "Simulate upload without actually uploading")
	cmd.Flags().BoolVar(&cfg.Upload.Resume, "resume", true, "Resume previous upload if interrupted")
	cmd.Flags().StringVar(&cfg.Upload.JournalPath, "journal", "", "Path to journal file for resumable uploads")
	cmd.Flags().BoolVar(&cfg.Upload.PreserveMetadata, "preserve-metadata", true, "Preserve file metadata as S3 object metadata")
	cmd.Flags().BoolVar(&cfg.Upload.SkipExisting, "skip-existing", true, "Skip files that already exist in the bucket")
	cmd.Flags().BoolP("glob", "g", false, "Treat input paths as glob patterns")

	// Mark required flags
	cmd.MarkFlagRequired("endpoint")
	cmd.MarkFlagRequired("bucket")
	cmd.MarkFlagRequired("access-key")
	cmd.MarkFlagRequired("secret-key")

	return cmd
}

func runUpload(ctx context.Context, cfg *config.Config, args []string, isGlob bool) error {
	// Initialize logger
	logger.SetLevel(cfg.LogLevel)

	// Initialize S3 client using the new package
	s3Config := s3client.Config{
		Endpoint:         cfg.S3.Endpoint,
		Region:           cfg.S3.Region,
		Bucket:           cfg.S3.Bucket,
		AccessKey:        cfg.S3.AccessKey,
		SecretKey:        cfg.S3.SecretKey,
		UseSSL:           cfg.S3.UseSSL,
		Prefix:           cfg.S3.Prefix,
		DisableChecksums: cfg.S3.DisableChecksums,
	}

	// Initialize journal for resumable uploads
	jnl := journal.New(cfg.Upload.JournalPath)
	if cfg.Upload.Resume {
		if err := jnl.Load(); err != nil {
			logger.Warn("Could not load journal: %v", err)
		}

		// Test if we can write to the journal file
		logger.Info("Testing journal write access...")
		if err := jnl.Save(); err != nil {
			logger.Error("Failed to write journal file: %v", err)
			logger.Warn("Continuing without journal - uploads will not be resumable")
		} else {
			logger.Info("Journal write test successful")
		}
	}

	// Start periodic save with context
	logger.Info("Starting periodic journal save")
	jnl.StartPeriodicSave(ctx)
	defer func() {
		logger.Info("Stopping periodic journal save")
		jnl.StopPeriodicSave()
		// Final save before exiting
		if err := jnl.Save(); err != nil {
			logger.Error("Failed to save journal before exit: %v", err)
		}
	}()

	// Create a wait group to wait for all uploads to complete
	var wg sync.WaitGroup
	var uploadErrors []error
	var errorsMutex sync.Mutex

	// Limit the number of concurrent archives being processed
	archiveSemaphore := make(chan struct{}, cfg.Upload.MaxConcurrentArchives)
	logger.Info("Processing up to %d archives simultaneously", cfg.Upload.MaxConcurrentArchives)

	// At the start of runUpload
	logger.Info("Starting upload process with PID: %d", os.Getpid())

	// Process each input path
	for _, path := range args {
		var filesToProcess []string

		if isGlob {
			// Handle as glob pattern
			logger.Debug("Processing pattern: %s", path)
			matches, err := filepath.Glob(path)
			if err != nil {
				logger.Error("Failed to expand glob pattern: %v", err)
				return fmt.Errorf("failed to expand glob pattern %s: %w", path, err)
			}

			logger.Debug("Glob pattern expanded to %d matches", len(matches))
			for i, match := range matches {
				logger.Debug("Match %d: %s", i+1, match)
			}

			if len(matches) == 0 {
				logger.Warn("No files matched pattern: %s", path)
				continue
			}

			logger.Info("Found %d files matching pattern: %s", len(matches), path)
			filesToProcess = matches
		} else {
			// If the path is a directory, find all zip files in it
			fileInfo, err := os.Stat(path)
			if err == nil && fileInfo.IsDir() {
				zipFiles, err := findZipFiles(path)
				if err != nil {
					return fmt.Errorf("failed to scan directory %s: %w", path, err)
				}

				if len(zipFiles) == 0 {
					logger.Warn("No zip files found in directory: %s", path)
					continue
				}

				logger.Info("Found %d zip files in directory: %s", len(zipFiles), path)
				filesToProcess = zipFiles
			} else {
				// Handle as literal path
				filesToProcess = []string{path}
			}
		}

		for _, filePath := range filesToProcess {
			// Capture filePath for the goroutine
			currentPath := filePath

			// Add to wait group
			wg.Add(1)

			// Acquire semaphore to limit concurrent archives
			archiveSemaphore <- struct{}{}

			// Process each file in a separate goroutine
			go func() {
				// Add panic recovery at the beginning
				defer func() {
					if r := recover(); r != nil {
						logger.Error("Panic recovered in archive processing: %v", r)
						// Still release resources
						<-archiveSemaphore
						wg.Done()
					}
				}()

				defer func() {
					// Release semaphore when done
					<-archiveSemaphore
					wg.Done()
					logger.Info("Released semaphore for archive: %s", filepath.Base(currentPath))
				}()

				// Log at the beginning of the goroutine
				archiveName := filepath.Base(currentPath)
				logger.Info("Started goroutine for archive: %s", archiveName)

				// Create a completely independent context for this archive
				archiveCtx, archiveCancel := context.WithCancel(context.Background())
				defer archiveCancel() // Ensure this context is cancelled when the goroutine exits

				logger.Info("Starting processing for archive: %s", archiveName)

				// Create a separate S3 client for each archive
				archiveS3Client, err := s3client.New(archiveCtx, s3Config)
				if err != nil {
					errorMsg := fmt.Errorf("failed to initialize S3 client for archive %s: %w", currentPath, err)
					logger.Error("%v", errorMsg)

					errorsMutex.Lock()
					uploadErrors = append(uploadErrors, errorMsg)
					errorsMutex.Unlock()
					return
				}

				// Determine if it's a zip file or directory
				isZip := filepath.Ext(currentPath) == ".zip"

				// Create Google Takeout adapter with archive-specific context
				takeout, err := googletakeout.New(archiveCtx, currentPath, isZip)
				if err != nil {
					errorMsg := fmt.Errorf("failed to process takeout at %s: %w", currentPath, err)
					logger.Error("%v", errorMsg)

					errorsMutex.Lock()
					uploadErrors = append(uploadErrors, errorMsg)
					errorsMutex.Unlock()
					return
				}

				// Create a separate worker pool for each file
				filePool := worker.NewPool(cfg.Upload.Concurrency)

				// Create a separate progress reporter for each archive
				archiveProgress := progress.New()

				// Create a separate journal for each archive if needed
				var archiveJournal *journal.Journal
				if cfg.Upload.JournalPath != "" {
					// Create a journal with a unique name for this archive
					journalPath := cfg.Upload.JournalPath
					if !strings.HasSuffix(journalPath, ".json") {
						journalPath = filepath.Join(journalPath, archiveName+".json")
					} else {
						// Insert archive name before .json extension
						ext := filepath.Ext(journalPath)
						base := strings.TrimSuffix(journalPath, ext)
						journalPath = base + "-" + archiveName + ext
					}

					logger.Info("Using journal at %s for archive: %s", journalPath, archiveName)
					archiveJournal = journal.New(journalPath)
					if cfg.Upload.Resume {
						if err := archiveJournal.Load(); err != nil {
							logger.Warn("Could not load journal for %s: %v", archiveName, err)
						}
					}

					// Start periodic save for this archive's journal
					archiveJournal.StartPeriodicSave(archiveCtx)
					defer archiveJournal.StopPeriodicSave()
				} else {
					// Use the main journal if no specific journal path was provided
					archiveJournal = jnl
				}

				// Start upload process with archive-specific resources
				logger.Info("Starting upload for archive: %s", archiveName)
				up := uploader.New(archiveCtx, archiveS3Client, takeout, archiveJournal, filePool, archiveProgress, cfg)

				if err := up.Run(); err != nil {
					errorMsg := fmt.Errorf("upload failed for %s: %w", currentPath, err)
					logger.Error("%v", errorMsg)

					errorsMutex.Lock()
					uploadErrors = append(uploadErrors, errorMsg)
					errorsMutex.Unlock()
				} else {
					logger.Info("Successfully completed upload for archive: %s", archiveName)
				}

				// Final log message
				logger.Info("Finished processing archive: %s", archiveName)
			}()
		}

		// Just before wg.Wait()
		logger.Info("About to wait for %d archives to complete", len(filesToProcess))
	}

	// Wait for all uploads to complete
	logger.Info("Waiting for all archives to complete...")
	wg.Wait()
	logger.Info("All archives have been processed")

	// Check if there were any errors
	if len(uploadErrors) > 0 {
		logger.Error("Encountered %d errors during upload", len(uploadErrors))
		for _, err := range uploadErrors {
			logger.Error("  %v", err)
		}
		return nil
	}

	return nil
}

func findZipFiles(dir string) ([]string, error) {
	var zipFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".zip" {
			zipFiles = append(zipFiles, path)
		}

		return nil
	})

	return zipFiles, err
}
