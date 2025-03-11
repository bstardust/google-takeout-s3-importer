# S3-Takeout-Upload

A command-line tool for uploading Google Takeout archives to S3-compatible storage services.

## Features

- Upload Google Takeout archives to any S3-compatible storage (AWS S3, Backblaze B2, MinIO, etc.)
- Concurrent uploads for improved performance
- Resumable uploads - continue where you left off if interrupted
- Preserve metadata from Google Takeout JSON files
- Extract and preserve EXIF metadata
- Progress reporting with ETA
- Support for both zipped and unzipped Google Takeout archives

## Installation

### Using Go

```bash
go install github.com/bstardust/google-takeout-s3-importer/cmd/s3-takeout-upload@latest
```

### Binary Releases

Download the appropriate binary for your platform from the [Releases](https://github.com/bstardust/google-takeout-s3-importer/releases) page.

## Usage

### Basic Usage

```bash
s3-takeout-upload upload \
  --endpoint=s3.amazonaws.com \
  --bucket=my-photos-bucket \
  --access-key=YOUR_ACCESS_KEY \
  --secret-key=YOUR_SECRET_KEY \
  path/to/takeout-*.zip
```

### Using MinIO or Other S3-Compatible Services

```bash
s3-takeout-upload upload \
  --endpoint=play.min.io \
  --bucket=my-bucket \
  --access-key=YOUR_ACCESS_KEY \
  --secret-key=YOUR_SECRET_KEY \
  --region=us-east-1 \
  path/to/takeout-folder
```

### Options
Global Flags:
--log-level string Log level (debug, info, warn, error) (default "info")
Upload Command Flags:
--endpoint string S3 endpoint URL (required)
--region string S3 region (default "us-east-1")
--bucket string S3 bucket name (required)
--access-key string S3 access key (required)
--secret-key string S3 secret key (required)
--use-ssl Use SSL for S3 connection (default true)
--prefix string Prefix for S3 object keys
--concurrency int Number of concurrent uploads (default 4)
--dry-run Simulate upload without actually uploading
--resume Resume previous upload if interrupted (default true)
--journal string Path to journal file for resumable uploads
--preserve-metadata Preserve file metadata as S3 object metadata (default true)
--skip-existing Skip files that already exist in the bucket (default true)


## Environment Variables

All command-line options can also be specified using environment variables with the `S3TAKEOUT_` prefix:

```bash
export S3TAKEOUT_ENDPOINT=s3.amazonaws.com
export S3TAKEOUT_BUCKET=my-photos-bucket
export S3TAKEOUT_ACCESS_KEY=YOUR_ACCESS_KEY
export S3TAKEOUT_SECRET_KEY=YOUR_SECRET_KEY

s3-takeout-upload upload path/to/takeout-*.zip
```

## License

MIT