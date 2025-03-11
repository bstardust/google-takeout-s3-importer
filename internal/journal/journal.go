// internal/journal/journal.go
package journal

import (
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
}

// UploadEntry represents a journal entry for an uploaded file
type UploadEntry struct {
	Path      string    `json:"path"`
	Uploaded  bool      `json:"uploaded"`
	Timestamp time.Time `json:"timestamp"`
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

	// Check if journal file exists
	if _, err := os.Stat(j.path); os.IsNotExist(err) {
		logger.Info("No journal file found at %s, starting fresh", j.path)
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
		return err
	}

	// Marshal journal
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}

	// Write journal file
	if err := os.WriteFile(j.path, data, 0644); err != nil {
		return err
	}

	logger.Debug("Saved journal with %d entries to %s", len(j.Uploads), j.path)

	return nil
}

// MarkUploaded marks a file as uploaded
func (j *Journal) MarkUploaded(path string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.Uploads[path] = UploadEntry{
		Path:      path,
		Uploaded:  true,
		Timestamp: time.Now(),
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
