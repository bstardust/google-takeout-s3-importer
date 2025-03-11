package s3client

import (
	"errors"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
)

// Common errors
var (
	ErrBucketNotFound     = errors.New("bucket not found")
	ErrObjectNotFound     = errors.New("object not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrConnectionFailed   = errors.New("connection failed")
)

// IsNotFoundError checks if an error is a "not found" error
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrBucketNotFound) || errors.Is(err, ErrObjectNotFound) {
		return true
	}

	// Check MinIO error
	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		switch minioErr.Code {
		case "NoSuchBucket", "NoSuchKey", "NotFound":
			return true
		}
	}

	// Check error string
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such")
}

// IsAuthError checks if an error is an authentication error
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrPermissionDenied) {
		return true
	}

	// Check MinIO error
	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		switch minioErr.Code {
		case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch", "AuthorizationHeaderMalformed":
			return true
		}
	}

	// Check error string
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "invalid credential") ||
		strings.Contains(errStr, "permission denied")
}

// FormatError formats an error for display
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	// Check if it's a MinIO error
	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		return fmt.Sprintf("S3 error: %s (code: %s)", minioErr.Message, minioErr.Code)
	}

	return err.Error()
}
