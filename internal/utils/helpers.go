package utils

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateS3BucketName checks if the provided S3 bucket name is valid according to AWS naming conventions.
func ValidateS3BucketName(bucketName string) error {
	if len(bucketName) < 3 || len(bucketName) > 63 {
		return errors.New("bucket name must be between 3 and 63 characters")
	}
	if strings.Contains(bucketName, " ") {
		return errors.New("bucket name cannot contain spaces")
	}
	if !isDNSCompatible(bucketName) {
		return errors.New("bucket name must be DNS compliant")
	}
	return nil
}

// isDNSCompatible checks if the bucket name is DNS compliant.
func isDNSCompatible(name string) bool {
	// Bucket names must be lowercase and can contain only letters, numbers, and hyphens.
	for _, char := range name {
		if !(char >= 'a' && char <= 'z') && !(char >= '0' && char <= '9') && char != '-' {
			return false
		}
	}
	return true
}

// ParseGoogleTakeoutURL validates and parses a Google Takeout URL.
func ParseGoogleTakeoutURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme != "https" {
		return nil, errors.New("URL must use HTTPS")
	}
	return parsedURL, nil
}