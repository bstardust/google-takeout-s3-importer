// internal/exif/exif.go
package exif

import (
	"io"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// Data represents EXIF metadata
type Data struct {
	DateTime *time.Time
	GPS      *GPSInfo
	Make     string
	Model    string
}

// GPSInfo represents GPS information from EXIF
type GPSInfo struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
}

// Extract extracts EXIF metadata from a reader
func Extract(r io.Reader) (*Data, error) {
	// Parse EXIF
	x, err := exif.Decode(r)
	if err != nil {
		return nil, err
	}

	data := &Data{}

	// Extract date/time
	if dt, err := x.DateTime(); err == nil {
		data.DateTime = &dt
	}

	// Extract GPS info
	if lat, long, err := x.LatLong(); err == nil {
		data.GPS = &GPSInfo{
			Latitude:  lat,
			Longitude: long,
		}

		// Try to get altitude
		if alt, err := x.Get(exif.GPSAltitude); err == nil {
			if rational, err := alt.Rat(0); err == nil {
				data.GPS.Altitude = float64(rational.Num().Int64()) / float64(rational.Denom().Int64())
			}
		}
	}

	// Extract camera info
	if make, err := x.Get(exif.Make); err == nil {
		if str, err := make.StringVal(); err == nil {
			data.Make = str
		}
	}

	if model, err := x.Get(exif.Model); err == nil {
		if str, err := model.StringVal(); err == nil {
			data.Model = str
		}
	}

	return data, nil
}
