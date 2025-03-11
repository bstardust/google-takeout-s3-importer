package fileinfo

import (
	"github.com/bstardust/google-takeout-s3-importer/pkg/s3client"
)

// IsMediaFile checks if a file is a media file (image or video)
func IsMediaFile(filename string) bool {
	return s3client.IsMediaFile(filename)
}

// IsImageFile checks if a file is an image
func IsImageFile(filename string) bool {
	return s3client.IsImageFile(filename)
}

// IsVideoFile checks if a file is a video
func IsVideoFile(filename string) bool {
	return s3client.IsVideoFile(filename)
}

// GetContentType returns the content type for a file
func GetContentType(filename string) string {
	return s3client.DetectContentType(filename)
}
