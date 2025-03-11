package s3client

import (
	"mime"
	"path/filepath"
	"strings"
)

// Common MIME types for various file extensions
var commonMimeTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".bmp":  "image/bmp",
	".heic": "image/heic",
	".heif": "image/heif",
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".avi":  "video/x-msvideo",
	".wmv":  "video/x-ms-wmv",
	".mkv":  "video/x-matroska",
	".json": "application/json",
	".txt":  "text/plain",
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".zip":  "application/zip",
	".tar":  "application/x-tar",
	".gz":   "application/gzip",
}

// DetectContentType determines the content type of a file based on its extension
func DetectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	// Check our common types first
	if mimeType, ok := commonMimeTypes[ext]; ok {
		return mimeType
	}

	// Fall back to the standard library
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		return mimeType
	}

	// Default to binary data
	return "application/octet-stream"
}

// IsImageFile checks if a file is an image based on its extension
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".tiff", ".tif", ".bmp", ".heic", ".heif":
		return true
	default:
		return false
	}
}

// IsVideoFile checks if a file is a video based on its extension
func IsVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp4", ".mov", ".avi", ".wmv", ".mkv", ".webm", ".flv", ".m4v", ".3gp":
		return true
	default:
		return false
	}
}

// IsMediaFile checks if a file is a media file (image or video)
func IsMediaFile(filename string) bool {
	return IsImageFile(filename) || IsVideoFile(filename)
}
