package cli

import (
	"context"
	"fmt"
	"path/filepath"

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
			return runUpload(cmd.Context(), cfg, args)
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

	// Upload options
	cmd.Flags().IntVar(&cfg.Upload.Concurrency, "concurrency", 4, "Number of concurrent uploads")
	cmd.Flags().BoolVar(&cfg.Upload.DryRun, "dry-run", false, "Simulate upload without actually uploading")
	cmd.Flags().BoolVar(&cfg.Upload.Resume, "resume", true, "Resume previous upload if interrupted")
	cmd.Flags().StringVar(&cfg.Upload.JournalPath, "journal", "", "Path to journal file for resumable uploads")
	cmd.Flags().BoolVar(&cfg.Upload.PreserveMetadata, "preserve-metadata", true, "Preserve file metadata as S3 object metadata")
	cmd.Flags().BoolVar(&cfg.Upload.SkipExisting, "skip-existing", true, "Skip files that already exist in the bucket")

	// Mark required flags
	cmd.MarkFlagRequired("endpoint")
	cmd.MarkFlagRequired("bucket")
	cmd.MarkFlagRequired("access-key")
	cmd.MarkFlagRequired("secret-key")

	return cmd
}

func runUpload(ctx context.Context, cfg *config.Config, args []string) error {
	// Initialize logger
	logger.SetLevel(cfg.LogLevel)

	// Initialize S3 client using the new package
	s3Config := s3client.Config{
		Endpoint:  cfg.S3.Endpoint,
		Region:    cfg.S3.Region,
		Bucket:    cfg.S3.Bucket,
		AccessKey: cfg.S3.AccessKey,
		SecretKey: cfg.S3.SecretKey,
		UseSSL:    cfg.S3.UseSSL,
		Prefix:    cfg.S3.Prefix,
	}

	s3Client, err := s3client.New(ctx, s3Config)
	if err != nil {
		return fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Initialize journal for resumable uploads
	jnl := journal.New(cfg.Upload.JournalPath)
	if cfg.Upload.Resume {
		if err := jnl.Load(); err != nil {
			logger.Warn("Could not load journal: %v", err)
		}
	}

	// Initialize worker pool
	pool := worker.NewPool(cfg.Upload.Concurrency)

	// Initialize progress reporter
	progressReporter := progress.New()

	// Process each input path
	for _, path := range args {
		// Determine if it's a zip file or directory
		isZip := filepath.Ext(path) == ".zip"

		// Create Google Takeout adapter
		takeout, err := googletakeout.New(ctx, path, isZip)
		if err != nil {
			return fmt.Errorf("failed to process takeout at %s: %w", path, err)
		}

		// Start upload process
		up := uploader.New(ctx, s3Client, takeout, jnl, pool, progressReporter, cfg)
		if err := up.Run(); err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}
	}

	return nil
}
