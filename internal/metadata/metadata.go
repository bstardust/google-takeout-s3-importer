package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bstardust/google-takeout-s3-importer/internal/exif"
	"github.com/bstardust/google-takeout-s3-importer/internal/logger"
)

// Metadata represents file metadata
type Metadata struct {
	Title            string      `json:"title,omitempty"`
	Description      string      `json:"description,omitempty"`
	CreationTime     *time.Time  `json:"creationTime,omitempty"`
	ModificationTime *time.Time  `json:"modificationTime,omitempty"`
	GeoData          *GeoData    `json:"geoData,omitempty"`
	CameraData       *CameraData `json:"cameraData,omitempty"`
	Tags             []string    `json:"tags,omitempty"`
	Albums           []string    `json:"albums,omitempty"`
	People           []string    `json:"people,omitempty"`
	Source           string      `json:"source,omitempty"`
}

// GeoData represents geographical data
type GeoData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
}

// CameraData represents camera information
type CameraData struct {
	Make  string `json:"make,omitempty"`
	Model string `json:"model,omitempty"`
}

// Extractor extracts metadata from files
type Extractor struct {
	timezone *time.Location
}

// NewExtractor creates a new metadata extractor
func NewExtractor(timezone *time.Location) *Extractor {
	if timezone == nil {
		timezone = time.UTC
	}
	return &Extractor{
		timezone: timezone,
	}
}

// ExtractFromJSON extracts metadata from a JSON file
func (e *Extractor) ExtractFromJSON(r io.Reader) (*Metadata, error) {
	var metadata Metadata
	if err := json.NewDecoder(r).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode JSON metadata: %w", err)
	}
	return &metadata, nil
}

// ExtractFromEXIF extracts metadata from EXIF data
func (e *Extractor) ExtractFromEXIF(r io.Reader) (*Metadata, error) {
	exifData, err := exif.Extract(r)
	if err != nil {
		return nil, fmt.Errorf("failed to extract EXIF data: %w", err)
	}

	metadata := &Metadata{}

	// Set creation time
	if exifData.DateTime != nil {
		metadata.CreationTime = exifData.DateTime
	}

	// Set geo data
	if exifData.GPS != nil {
		metadata.GeoData = &GeoData{
			Latitude:  exifData.GPS.Latitude,
			Longitude: exifData.GPS.Longitude,
			Altitude:  exifData.GPS.Altitude,
		}
	}

	// Set camera data
	if exifData.Make != "" || exifData.Model != "" {
		metadata.CameraData = &CameraData{
			Make:  exifData.Make,
			Model: exifData.Model,
		}
	}

	return metadata, nil
}

// ExtractFromFile extracts metadata from a file
func (e *Extractor) ExtractFromFile(fsys fs.FS, path string) (*Metadata, error) {
	// First, check if there's a corresponding JSON metadata file
	jsonPath := path + ".json"
	jsonExists, _ := Exists(fsys, jsonPath)

	var metadata *Metadata

	if jsonExists {
		// Extract metadata from JSON
		jsonFile, err := fsys.Open(jsonPath)
		if err != nil {
			logger.Warn("Failed to open JSON metadata file %s: %v", jsonPath, err)
		} else {
			defer jsonFile.Close()
			metadata, err = e.ExtractFromJSON(jsonFile)
			if err != nil {
				logger.Warn("Failed to extract metadata from JSON file %s: %v", jsonPath, err)
			}
		}
	}

	// If no metadata from JSON or incomplete, try EXIF
	if metadata == nil {
		metadata = &Metadata{}
	}

	// Try to extract EXIF data
	file, err := fsys.Open(path)
	if err != nil {
		return metadata, nil // Return what we have so far
	}
	defer file.Close()

	exifMetadata, err := e.ExtractFromEXIF(file)
	if err != nil {
		return metadata, nil // Return what we have so far
	}

	// Merge EXIF metadata with JSON metadata (JSON takes precedence)
	e.mergeMetadata(metadata, exifMetadata)

	// Set title from filename if not set
	if metadata.Title == "" {
		metadata.Title = filepath.Base(path)
	}

	return metadata, nil
}

// mergeMetadata merges two metadata objects
func (e *Extractor) mergeMetadata(target, source *Metadata) {
	if target.Title == "" {
		target.Title = source.Title
	}
	if target.Description == "" {
		target.Description = source.Description
	}
	if target.CreationTime == nil {
		target.CreationTime = source.CreationTime
	}
	if target.ModificationTime == nil {
		target.ModificationTime = source.ModificationTime
	}
	if target.GeoData == nil {
		target.GeoData = source.GeoData
	}
	if target.CameraData == nil {
		target.CameraData = source.CameraData
	}
	if len(target.Tags) == 0 {
		target.Tags = source.Tags
	}
	if len(target.Albums) == 0 {
		target.Albums = source.Albums
	}
	if len(target.People) == 0 {
		target.People = source.People
	}
	if target.Source == "" {
		target.Source = source.Source
	}
}

// ToMap converts metadata to a map for S3 object metadata
func (m *Metadata) ToMap() map[string]string {
	result := make(map[string]string)

	if m.Title != "" {
		result["title"] = m.Title
	}
	if m.Description != "" {
		result["description"] = m.Description
	}
	if m.CreationTime != nil {
		result["creation-time"] = m.CreationTime.Format(time.RFC3339)
	}
	if m.ModificationTime != nil {
		result["modification-time"] = m.ModificationTime.Format(time.RFC3339)
	}
	if m.GeoData != nil {
		result["geo-latitude"] = fmt.Sprintf("%f", m.GeoData.Latitude)
		result["geo-longitude"] = fmt.Sprintf("%f", m.GeoData.Longitude)
		if m.GeoData.Altitude != 0 {
			result["geo-altitude"] = fmt.Sprintf("%f", m.GeoData.Altitude)
		}
	}
	if m.CameraData != nil {
		if m.CameraData.Make != "" {
			result["camera-make"] = m.CameraData.Make
		}
		if m.CameraData.Model != "" {
			result["camera-model"] = m.CameraData.Model
		}
	}
	if len(m.Tags) > 0 {
		result["tags"] = strings.Join(m.Tags, ",")
	}
	if len(m.Albums) > 0 {
		result["albums"] = strings.Join(m.Albums, ",")
	}
	if len(m.People) > 0 {
		result["people"] = strings.Join(m.People, ",")
	}
	if m.Source != "" {
		result["source"] = m.Source
	}

	return result
}

// Exists checks if a path exists in a filesystem
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
