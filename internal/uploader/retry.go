package uploader

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
)

// RetryConfig defines retry behavior for operations that might fail transiently
type RetryConfig struct {
	// MaxRetries is the maximum number of retries before giving up
	MaxRetries int

	// InitialBackoff is the duration to wait before the first retry
	InitialBackoff time.Duration

	// MaxBackoff is the maximum duration to wait between retries
	MaxBackoff time.Duration

	// BackoffFactor is the factor by which to increase backoff after each retry
	BackoffFactor float64

	// RetryableErrors is a map of error types that should be retried
	RetryableErrors map[string]bool
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      5,
		InitialBackoff:  1 * time.Second,
		MaxBackoff:      1 * time.Minute,
		BackoffFactor:   2.0,
		RetryableErrors: defaultRetryableErrors(),
	}
}

// defaultRetryableErrors returns a map of common S3 error codes that should be retried
func defaultRetryableErrors() map[string]bool {
	return map[string]bool{
		"RequestTimeout":         true,
		"RequestTimeTooSkewed":   true,
		"InternalError":          true,
		"SlowDown":               true,
		"OperationAborted":       true,
		"ConnectionError":        true,
		"NetworkingError":        true,
		"ThrottlingException":    true,
		"ServiceUnavailable":     true,
		"RequestLimitExceeded":   true,
		"BandwidthLimitExceeded": true,
		"IDPCommunicationError":  true,
		"KMSTemporaryFailure":    true,
		"KMSThrottlingException": true,
		"PermanentRedirect":      true,
		"TemporaryRedirect":      true,
		"ServerSideEncryptionConfigurationNotFoundError": true,
	}
}

// IsRetryable determines if an error should be retried based on its type or message
func (rc RetryConfig) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a context cancellation or deadline exceeded
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check common S3 error types
	for errCode := range rc.RetryableErrors {
		if strings.Contains(err.Error(), errCode) {
			return true
		}
	}

	// Check for common transient error patterns
	lowerErr := strings.ToLower(err.Error())
	if strings.Contains(lowerErr, "timeout") ||
		strings.Contains(lowerErr, "connection") ||
		strings.Contains(lowerErr, "reset") ||
		strings.Contains(lowerErr, "broken pipe") ||
		strings.Contains(lowerErr, "network") ||
		strings.Contains(lowerErr, "unavailable") {
		return true
	}

	return false
}

// RetryWithBackoff retries the given operation with exponential backoff
func RetryWithBackoff(ctx context.Context, operation string, fn func() error, config RetryConfig) error {
	var err error
	var attempt int

	for attempt = 0; attempt <= config.MaxRetries; attempt++ {
		// Check if context is done before attempting operation
		if ctx.Err() != nil {
			return fmt.Errorf("%s canceled: %w", operation, ctx.Err())
		}

		// If this is a retry, log the attempt
		if attempt > 0 {
			logger.Debug("Retry attempt %d/%d for %s", attempt, config.MaxRetries, operation)
		}

		// Attempt the operation
		err = fn()

		// Success! Return nil
		if err == nil {
			if attempt > 0 {
				logger.Info("Successfully completed %s after %d retries", operation, attempt)
			}
			return nil
		}

		// Check if the error is retryable
		if !config.IsRetryable(err) {
			logger.Warn("Non-retryable error for %s: %v", operation, err)
			return err
		}

		// Last attempt failed
		if attempt == config.MaxRetries {
			break
		}

		// Calculate backoff duration
		backoff := getBackoffDuration(attempt, config)

		// Log the backoff
		logger.Debug("Backing off for %v before retrying %s: %v", backoff, operation, err)

		// Wait for the backoff duration or until context is canceled
		select {
		case <-time.After(backoff):
			// Continue to the next attempt
		case <-ctx.Done():
			return fmt.Errorf("%s canceled during retry: %w", operation, ctx.Err())
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operation, attempt, err)
}

// getBackoffDuration calculates the backoff duration for a retry attempt
func getBackoffDuration(attempt int, config RetryConfig) time.Duration {
	// Calculate exponential backoff
	backoff := float64(config.InitialBackoff) * math.Pow(config.BackoffFactor, float64(attempt))

	// Add jitter (±20% randomness)
	jitter := (rand.Float64() * 0.4) - 0.2
	backoff = backoff * (1 + jitter)

	// Ensure backoff doesn't exceed max
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	return time.Duration(backoff)
}
