// internal/progress/reporter.go
package progress

import (
	"sync"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
)

// Reporter tracks and reports upload progress
type Reporter struct {
	mu             sync.Mutex
	total          int
	completed      int
	skipped        int
	errors         int
	startTime      time.Time
	lastUpdateTime time.Time
	updateInterval time.Duration
}

// New creates a new progress reporter
func New() *Reporter {
	return &Reporter{
		updateInterval: 2 * time.Second,
	}
}

// Start initializes the progress reporter with the total number of files
func (r *Reporter) Start(total int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.total = total
	r.completed = 0
	r.skipped = 0
	r.errors = 0
	r.startTime = time.Now()
	r.lastUpdateTime = time.Now()

	logger.Info("Starting upload of %d files", total)
}

// Complete marks a file as successfully uploaded
func (r *Reporter) Complete(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.completed++
	r.updateProgress()
}

// Skip marks a file as skipped
func (r *Reporter) Skip(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skipped++
	r.updateProgress()
}

// Error marks a file as failed
func (r *Reporter) Error(path string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.errors++
	r.updateProgress()
}

// Finish completes the progress reporting
func (r *Reporter) Finish() {
	r.mu.Lock()
	defer r.mu.Unlock()

	duration := time.Since(r.startTime)

	logger.Info("Upload complete: %d/%d files uploaded, %d skipped, %d errors in %s",
		r.completed, r.total, r.skipped, r.errors, duration.Round(time.Second))
}

// updateProgress updates and displays the progress
func (r *Reporter) updateProgress() {
	now := time.Now()
	if now.Sub(r.lastUpdateTime) < r.updateInterval {
		return
	}

	r.lastUpdateTime = now
	duration := now.Sub(r.startTime)
	processed := r.completed + r.skipped + r.errors

	if processed == 0 {
		return
	}

	percentage := float64(processed) / float64(r.total) * 100

	// Calculate estimated time remaining
	var eta string
	if r.completed > 0 {
		timePerFile := duration / time.Duration(processed)
		remaining := timePerFile * time.Duration(r.total-processed)
		eta = remaining.Round(time.Second).String()
	} else {
		eta = "unknown"
	}

	logger.Info("Progress: %.1f%% (%d/%d, %d completed, %d skipped, %d errors) ETA: %s",
		percentage, processed, r.total, r.completed, r.skipped, r.errors, eta)
}
