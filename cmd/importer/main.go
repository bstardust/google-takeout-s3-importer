package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/google-takeout-s3-importer/internal/config"
	"github.com/google-takeout-s3-importer/internal/s3"
	"github.com/google-takeout-s3-importer/internal/takeout"
	"github.com/google-takeout-s3-importer/internal/utils"
)

func main() {
	var configFile string
	var bucketName string
	var takeoutPath string

	rootCmd := &cobra.Command{
		Use:   "importer",
		Short: "Import media from Google Takeout to S3",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				utils.LogError(err)
				os.Exit(1)
			}

			takeoutFiles, err := takeout.ParseTakeout(takeoutPath)
			if err != nil {
				utils.LogError(err)
				os.Exit(1)
			}

			s3Client, err := s3.NewClient(cfg.S3)
			if err != nil {
				utils.LogError(err)
				os.Exit(1)
			}

			for _, file := range takeoutFiles {
				err := s3Client.Upload(file, bucketName)
				if err != nil {
					utils.LogError(err)
				} else {
					fmt.Printf("Successfully uploaded %s to %s\n", file.Name, bucketName)
				}
			}
		},
	}

	rootCmd.Flags().StringVar(&configFile, "config", "", "Path to the configuration file")
	rootCmd.Flags().StringVar(&bucketName, "bucket", "", "S3 bucket name")
	rootCmd.Flags().StringVar(&takeoutPath, "takeout", "", "Path to Google Takeout files")

	if err := rootCmd.Execute(); err != nil {
		utils.LogError(err)
		os.Exit(1)
	}
}