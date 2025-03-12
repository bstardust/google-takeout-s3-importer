// internal/journal/journal.go
package journal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
)

// Journal tracks upload progress for resumability
type Journal struct {
	mu           sync.Mutex
	path         string
	Uploads      map[string]UploadEntry `json:"uploads"`
	lastSaveTime time.Time
	saveInterval time.Duration
	batchCount   int
	cancelSave   context.CancelFunc // Add this to cancel the goroutine
}

// UploadEntry represents a journal entry for an uploaded file
type UploadEntry struct {
	Path      string    `json:"path"`
	Uploaded  bool      `json:"uploaded"`
	Timestamp time.Time `json:"timestamp"`
	Archive   string    `json:"archive"`
}

// New creates a new journal
func New(path string) *Journal {
	if path == "" {
		// Use default path in user's home directory
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, ".s3-takeout-upload-journal.json")
		} else {
			path = ".s3-takeout-upload-journal.json"
		}
	}

	logger.Info("Creating journal with path: %s", path)

	// Create an empty file if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		logger.Info("Journal file doesn't exist, creating empty file at %s", path)

		// Create directory if it doesn't exist
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Error("Failed to create journal directory %s: %v", dir, err)
		} else {
			// Create empty file
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
			if err != nil {
				logger.Error("Failed to create empty journal file: %v", err)
			} else {
				file.Close()
				logger.Info("Successfully created empty journal file")
			}
		}
	}

	return &Journal{
		path:         path,
		Uploads:      make(map[string]UploadEntry),
		saveInterval: 30 * time.Second,
	}
}

// Load loads the journal from disk
func (j *Journal) Load() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	logger.Info("Attempting to load journal from %s", j.path)

	// Check if journal file exists
	if _, err := os.Stat(j.path); os.IsNotExist(err) {
		logger.Info("No journal file found at %s, starting fresh", j.path)
		// Try to create an empty journal file immediately
		if err := j.Save(); err != nil {
			logger.Error("Failed to create initial journal file: %v", err)
		}
		return nil
	}

	// Read journal file
	data, err := os.ReadFile(j.path)
	if err != nil {
		return err
	}

	// Parse journal
	var journal Journal
	if err := json.Unmarshal(data, &journal); err != nil {
		return err
	}

	j.Uploads = journal.Uploads
	logger.Info("Loaded journal with %d entries from %s", len(j.Uploads), j.path)

	return nil
}

// 3. Add a method to start the periodic save with context
func (j *Journal) StartPeriodicSave(ctx context.Context) {
	// Create a child context we can cancel
	saveCtx, cancel := context.WithCancel(ctx)
	j.cancelSave = cancel

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := j.Save(); err != nil {
					logger.Error("Failed to perform periodic journal save: %v", err)
				} else {
					logger.Info("Performed periodic journal save with %d entries", len(j.Uploads))
				}
			case <-saveCtx.Done():
				logger.Debug("Stopping periodic journal save")
				return
			}
		}
	}()
	logger.Debug("Started periodic journal save")
}

// 4. Add a method to stop the periodic save
func (j *Journal) StopPeriodicSave() {
	if j.cancelSave != nil {
		j.cancelSave()
		j.cancelSave = nil
	}
}

// Save saves the journal to disk
func (j *Journal) Save() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	if now.Sub(j.lastSaveTime) < j.saveInterval && len(j.Uploads) > 0 {
		return nil // Don't save too frequently
	}

	j.lastSaveTime = now

	// Create directory if it doesn't exist
	dir := filepath.Dir(j.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("Failed to create journal directory: %v", err)
		return err
	}

	// Marshal journal
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal journal: %v", err)
		return err
	}

	// Write journal file
	if err := os.WriteFile(j.path, data, 0644); err != nil {
		logger.Error("Failed to write journal file: %v", err)
		return err
	}

	logger.Info("Saved journal with %d entries to %s", len(j.Uploads), j.path)
	return nil
}

// MarkUploaded marks a file as uploaded
func (j *Journal) MarkUploaded(path string, archive string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.Uploads[path] = UploadEntry{
		Path:      path,
		Uploaded:  true,
		Timestamp: time.Now(),
		Archive:   archive,
	}

	// Save after every 100 files
	j.batchCount++
	if j.batchCount >= 100 {
		j.batchCount = 0
		go j.Save() // Save in background to not block
	}
}

// IsUploaded checks if a file has been uploaded
func (j *Journal) IsUploaded(path string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()

	entry, exists := j.Uploads[path]
	return exists && entry.Uploaded
}

// Clear clears the journal
func (j *Journal) Clear() {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.Uploads = make(map[string]UploadEntry)
	j.Save()
}

// Stats returns statistics about the journal
func (j *Journal) Stats() (total int, uploaded int) {
	j.mu.Lock()
	defer j.mu.Unlock()

	total = len(j.Uploads)
	for _, entry := range j.Uploads {
		if entry.Uploaded {
			uploaded++
		}
	}

	return total, uploaded
}

// ListCompleted returns a list of all completed uploads
func (j *Journal) ListCompleted() []string {
	j.mu.Lock()
	defer j.mu.Unlock()

	var completed []string
	for path, entry := range j.Uploads {
		if entry.Uploaded {
			completed = append(completed, path)
		}
	}
	return completed
}
