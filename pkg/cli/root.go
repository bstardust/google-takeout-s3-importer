// pkg/cli/root.go
package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bstardust/google-takeout-s3-importer/internal/config"
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/spf13/cobra"
)

func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interruption signals
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		logger.Info("Received interrupt signal, shutting down gracefully...")
		cancel()
	}()

	rootCmd := &cobra.Command{
		Use:   "s3-takeout-upload",
		Short: "Upload Google Takeout archives to S3-compatible storage",
		Long:  `A tool for uploading Google Takeout archives to S3-compatible storage services like AWS S3, Backblaze B2, MinIO, etc.`,
	}

	// Global flags
	config := config.New()
	rootCmd.PersistentFlags().StringVar(&config.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	// Add commands
	rootCmd.AddCommand(newUploadCommand(ctx, config))

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		logger.Error("Error executing command: %v", err)
		os.Exit(1)
	}
}
