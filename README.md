# google-takeout-s3-importer/google-takeout-s3-importer/README.md

# Google Takeout to S3 Importer

This project provides a command-line tool to import media files from Google Takeout into an Amazon S3 bucket. It allows users to specify the necessary details for the import process, including S3 credentials and the path to the Google Takeout files.

## Features

- Import media files from Google Takeout.
- Upload files to an Amazon S3 bucket.
- Command-line interface for easy user interaction.
- Configuration management for S3 credentials and file paths.

## Prerequisites

- Go 1.16 or later
- AWS account with S3 access
- Google Takeout files

## Installation

1. Clone the repository:

   ```
   git clone https://github.com/yourusername/google-takeout-s3-importer.git
   ```

2. Navigate to the project directory:

   ```
   cd google-takeout-s3-importer
   ```

3. Install the dependencies:

   ```
   go mod tidy
   ```

## Usage

To run the importer, use the following command:

```
go run cmd/importer/main.go --s3-bucket <bucket-name> --takeout-path <path-to-takeout>
```

### Flags

- `--s3-bucket`: The name of the S3 bucket where the media files will be uploaded.
- `--takeout-path`: The local path to the Google Takeout files.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any enhancements or bug fixes.

## License

This project is licensed under the MIT License. See the LICENSE file for more details.