package googletakeout

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/fileinfo"
	"github.com/bstardust/google-takeout-s3-importer/internal/fshelper"
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
	"github.com/bstardust/google-takeout-s3-importer/internal/metadata"
)

// Takeout represents a Google Takeout archive
type Takeout struct {
	fsys       fs.FS
	mediaFiles map[string]*MediaFile
	extractor  *metadata.Extractor
}

// MediaFile represents a media file in the takeout
type MediaFile struct {
	Path     string
	Metadata *metadata.Metadata
	Size     int64
}

// New creates a new Takeout adapter
func New(ctx context.Context, path string, isZip bool) (*Takeout, error) {
	var fsys fs.FS
	var err error

	if isZip {
		fsys, err = fshelper.OpenZip(path)
	} else {
		fsys = os.DirFS(path)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open takeout: %w", err)
	}

	t := &Takeout{
		fsys:       fsys,
		mediaFiles: make(map[string]*MediaFile),
		extractor:  metadata.NewExtractor(time.UTC),
	}

	if err := t.scanTakeout(ctx); err != nil {
		return nil, err
	}

	return t, nil
}

// scanTakeout scans the takeout archive and builds the media file index
func (t *Takeout) scanTakeout(ctx context.Context) error {
	// Walk through the filesystem
	return fshelper.WalkDir(t.fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if d.IsDir() {
			return nil
		}

		// Check if it's a media file
		if fileinfo.IsMediaFile(path) && !strings.HasSuffix(path, ".json") {
			info, err := d.Info()
			if err != nil {
				logger.Warn("Failed to get file info for %s: %v", path, err)
				return nil
			}

			t.mediaFiles[path] = &MediaFile{
				Path: path,
				Size: info.Size(),
			}

			// Extract metadata
			meta, err := t.extractor.ExtractFromFile(t.fsys, path)
			if err != nil {
				logger.Warn("Failed to extract metadata for %s: %v", path, err)
			} else {
				t.mediaFiles[path].Metadata = meta
			}
		}

		return nil
	})
}

// ListFiles returns all media files in the takeout
func (t *Takeout) ListFiles() []*MediaFile {
	files := make([]*MediaFile, 0, len(t.mediaFiles))
	for _, file := range t.mediaFiles {
		files = append(files, file)
	}
	return files
}

// OpenFile opens a file from the takeout
func (t *Takeout) OpenFile(path string) (io.ReadCloser, error) {
	file, err := t.fsys.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// GetMetadata returns the metadata for a file
func (t *Takeout) GetMetadata(path string) *metadata.Metadata {
	if file, ok := t.mediaFiles[path]; ok {
		return file.Metadata
	}
	return nil
}

// GetSize returns the size of a file
func (t *Takeout) GetSize(path string) int64 {
	if file, ok := t.mediaFiles[path]; ok {
		return file.Size
	}
	return 0
}
