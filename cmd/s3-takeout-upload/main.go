// cmd/s3-takeout-upload/main.go
package main

import (
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/bstardust/google-takeout-s3-importer/pkg/cli"
)

func main() {
	// Initialize logger
	logger.Init()

	// Execute CLI
	cli.Execute()
}
