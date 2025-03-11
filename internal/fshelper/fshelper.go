package fshelper

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// NameFS is a filesystem that has a name
type NameFS interface {
	fs.FS
	Name() string
}

// DirFS represents a directory filesystem with a name
type DirFS struct {
	fs.FS
	name string
}

// Name returns the name of the filesystem
func (d *DirFS) Name() string {
	return d.name
}

// ZipFS represents a zip filesystem with a name
type ZipFS struct {
	*zip.Reader
	name string
	rc   io.Closer
}

// Name returns the name of the filesystem
func (z *ZipFS) Name() string {
	return z.name
}

// Close closes the zip file
func (z *ZipFS) Close() error {
	if z.rc != nil {
		return z.rc.Close()
	}
	return nil
}

// ParsePath parses a list of paths and returns a list of filesystems
func ParsePath(paths []string) ([]fs.FS, error) {
	var fsyss []fs.FS

	for _, path := range paths {
		// Check if the path is a glob pattern
		matches, err := filepath.Glob(path)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %s: %w", path, err)
		}

		if len(matches) == 0 {
			// No matches, try as a direct path
			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("path does not exist: %s", path)
				}
				return nil, fmt.Errorf("error accessing path %s: %w", path, err)
			}
			matches = []string{path}
		}

		for _, match := range matches {
			// Check if it's a directory or a zip file
			info, err := os.Stat(match)
			if err != nil {
				return nil, fmt.Errorf("error accessing path %s: %w", match, err)
			}

			if info.IsDir() {
				// It's a directory
				fsys := os.DirFS(match)
				fsyss = append(fsyss, &DirFS{
					FS:   fsys,
					name: filepath.Base(match),
				})
			} else if strings.HasSuffix(strings.ToLower(match), ".zip") {
				// It's a zip file
				zipFS, err := OpenZip(match)
				if err != nil {
					return nil, fmt.Errorf("error opening zip file %s: %w", match, err)
				}
				fsyss = append(fsyss, zipFS)
			} else {
				return nil, fmt.Errorf("unsupported file type: %s", match)
			}
		}
	}

	return fsyss, nil
}

// OpenZip opens a zip file and returns a filesystem
func OpenZip(path string) (fs.FS, error) {
	zipFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening zip file: %w", err)
	}

	info, err := zipFile.Stat()
	if err != nil {
		zipFile.Close()
		return nil, fmt.Errorf("error getting zip file info: %w", err)
	}

	zipReader, err := zip.NewReader(zipFile, info.Size())
	if err != nil {
		zipFile.Close()
		return nil, fmt.Errorf("error creating zip reader: %w", err)
	}

	return &ZipFS{
		Reader: zipReader,
		name:   filepath.Base(path),
		rc:     zipFile,
	}, nil
}

// WalkDir walks a filesystem and calls the function for each file
func WalkDir(fsys fs.FS, root string, fn func(path string, d fs.DirEntry, err error) error) error {
	return fs.WalkDir(fsys, root, fn)
}

// ReadFile reads a file from a filesystem
func ReadFile(fsys fs.FS, name string) ([]byte, error) {
	return fs.ReadFile(fsys, name)
}

// Open opens a file from a filesystem
func Open(fsys fs.FS, name string) (fs.File, error) {
	return fsys.Open(name)
}

// IsDir checks if a path is a directory
func IsDir(fsys fs.FS, path string) (bool, error) {
	info, err := fs.Stat(fsys, path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// Exists checks if a path exists
func Exists(fsys fs.FS, path string) (bool, error) {
	_, err := fs.Stat(fsys, path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
