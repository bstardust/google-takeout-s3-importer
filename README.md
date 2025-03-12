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
- Automatic retries with exponential backoff for transient errors
- Dry run mode for testing without actual uploads

## Installation

### Using Go

```bash
go install github.com/bstardust/google-takeout-s3-importer/cmd/s3-takeout-upload@latest
```

Run the following command to compile the code from the root directory:

```bash
go build -o s3-takeout-upload ./cmd/s3-takeout-upload
```

Add the s3-takeout-upload to your command, mac example below:
```bash
sudo mv s3-takeout-upload /usr/local/bin/
```

<!-- ### Binary Releases

Download the appropriate binary for your platform from the [Releases](https://github.com/bstardust/google-takeout-s3-importer/releases) page. -->

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

### Using Backblaze B2 Example

```bash
s3-takeout-upload upload \
  --endpoint=s3.us-west-004.backblazeb2.com \
  --bucket=my-bucket \
  --access-key=YOUR_KEY_ID \
  --secret-key=YOUR_APPLICATION_KEY \
  --disable-checksums
  path/to/takeout-folder
```

### Using Dry Run Mode

Test the upload process without actually transferring files:

```bash
s3-takeout-upload upload \
  --endpoint=s3.amazonaws.com \
  --bucket=my-photos-bucket \
  --access-key=YOUR_ACCESS_KEY \
  --secret-key=YOUR_SECRET_KEY \
  --dry-run \
  path/to/takeout-*.zip
```

### Custom Path Prefix

Store files under a specific prefix in your bucket:

```bash
s3-takeout-upload upload \
  --endpoint=s3.amazonaws.com \
  --bucket=my-photos-bucket \
  --access-key=YOUR_ACCESS_KEY \
  --secret-key=YOUR_SECRET_KEY \
  --prefix=google-photos/2022 \
  path/to/takeout-*.zip
```

### Options

#### Global Flags:
| Flag | Description | Default |
|------|-------------|---------|
| `--log-level` | Log level (debug, info, warn, error) | info |

#### Upload Command Flags:
| Flag | Description | Default |
|------|-------------|---------|
| `--endpoint` | S3 endpoint URL | (required) |
| `--region` | S3 region | us-east-1 |
| `--bucket` | S3 bucket name | (required) |
| `--access-key` | S3 access key | (required) |
| `--secret-key` | S3 secret key | (required) |
| `--use-ssl` | Use SSL for S3 connection | true |
| `--prefix` | Prefix for S3 object keys | |
| `--concurrency` | Number of concurrent uploads | 4 |
| `--dry-run` | Simulate upload without actually uploading | false |
| `--resume` | Resume previous upload if interrupted | true |
| `--journal` | Path to journal file for resumable uploads | |
| `--preserve-metadata` | Preserve file metadata as S3 object metadata | true |
| `--skip-existing` | Skip files that already exist in the bucket | true |
| `--disable-checksums` | Disable checksum verification for compatibility with certain S3 services (like Backblaze B2) | false |

## Environment Variables

All command-line options can also be specified using environment variables with the `S3TAKEOUT_` prefix:

```bash
export S3TAKEOUT_ENDPOINT=s3.amazonaws.com
export S3TAKEOUT_BUCKET=my-photos-bucket
export S3TAKEOUT_ACCESS_KEY=YOUR_ACCESS_KEY
export S3TAKEOUT_SECRET_KEY=YOUR_SECRET_KEY

s3-takeout-upload upload path/to/takeout-*.zip
```

## Metadata Handling

This tool preserves metadata from several sources:

1. **Google Takeout JSON files** - Each media file in Google Takeout typically has an accompanying JSON file with metadata
2. **EXIF data** - For image files, EXIF metadata is extracted directly from the files
3. **File attributes** - Basic information like creation time and modification time

Preserved metadata includes:
- Creation and modification times
- Geolocation data (latitude, longitude, altitude)
- Camera information (make, model)
- Photo titles and descriptions
- Album information
- People tags

This metadata is stored as S3 object metadata and can be retrieved when downloading files from S3.

## Error Handling and Retries

The tool automatically retries operations that fail due to transient errors such as:
- Network timeouts
- S3 service temporary unavailability
- Rate limiting

Retries use exponential backoff with jitter to avoid overwhelming services during recovery.
For detailed information about retries, use the `--log-level=debug` option.

## Troubleshooting

### Common Issues

1. **Connection failures**:
   - Verify your endpoint and credentials
   - Check network connectivity
   - Ensure the bucket exists and is accessible

2. **Slow uploads**:
   - Increase concurrency with `--concurrency=8` (or higher)
   - Check your network bandwidth
   - Consider using a geographically closer S3 endpoint

3. **Failed uploads with checksum errors** (especially with Backblaze B2):
   - Use the `--disable-checksums` flag to switch to AWS SDK client for uploads
   - This is particularly helpful for large files or video files that may have checksum verification issues
   - Example error: "SignatureDoesNotMatch" or "InvalidDigest" errors from B2

4. **Missing files**:
   - Use `--log-level=debug` to see which files are being processed
   - Check if files were skipped due to `--skip-existing`
   - Verify the input path contains the expected files

For more help, check the detailed logs or open an issue on the GitHub repository.

## License

MIT