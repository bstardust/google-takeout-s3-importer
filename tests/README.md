# Testing Guide for Google Takeout S3 Importer

This directory contains integration tests for the Google Takeout S3 Importer project. This README provides detailed information on how to run tests, set up test environments, and integrate testing into your development workflow.

## Test Structure

The project uses the following test structure:

- **Unit Tests**: Located alongside the code they test with `_test.go` suffix
  - `internal/uploader/uploader_test.go`: Tests for the uploader component
  - `pkg/s3client/client_test.go`: Tests for the S3 client
- **Integration Tests**: Located in this directory
  - `tests/integration_test.go`: End-to-end tests with actual S3 storage

## Running Tests

### Unit Tests

Run unit tests for specific packages:

```bash
# Run uploader tests
go test ./internal/uploader

# Run S3 client tests
go test ./pkg/s3client

# Run with verbose output
go test -v ./internal/uploader

# Run all unit tests
go test ./internal/... ./pkg/...
```

### Integration Tests

Integration tests require a running S3-compatible server and environment variables:

```bash
# Run integration tests with minimal configuration
INTEGRATION_TEST=true go test ./tests

# Run with custom S3 configuration -- EXAMPLE BELOW
INTEGRATION_TEST=true \
TEST_S3_ENDPOINT=play.min.io \
TEST_S3_BUCKET=my-test-bucket \
TEST_S3_ACCESS_KEY=Q3AM3UQ867SPQQA43P2F \
TEST_S3_SECRET_KEY=zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG \
TEST_S3_REGION=us-east-1 \
TEST_S3_USE_SSL=true \
TEST_TAKEOUT_PATH=./testdata/takeout \
go test -v ./tests
```

#### Required Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `INTEGRATION_TEST` | Set to "true" to enable integration tests | - |
| `TEST_S3_ENDPOINT` | S3 endpoint URL | localhost:9000 |
| `TEST_S3_BUCKET` | S3 bucket name | test-bucket |
| `TEST_S3_ACCESS_KEY` | S3 access key | minioadmin |
| `TEST_S3_SECRET_KEY` | S3 secret key | minioadmin |
| `TEST_S3_REGION` | S3 region | us-east-1 |
| `TEST_S3_USE_SSL` | Use SSL for S3 connection | false |
| `TEST_TAKEOUT_PATH` | Path to test Google Takeout data | ./testdata/takeout |

### Setting Up a Local Test Environment

For local integration testing, you can run MinIO in Docker:

```bash
# Start MinIO server
docker run -d -p 9000:9000 -p 9001:9001 \
  -e "MINIO_ROOT_USER=minioadmin" \
  -e "MINIO_ROOT_PASSWORD=minioadmin" \
  --name minio-test \
  minio/minio server /data --console-address ":9001"

# Create test bucket
docker exec minio-test mkdir -p /data/test-bucket

# Run integration tests using local MinIO
INTEGRATION_TEST=true go test ./tests
```

## Test Data

The integration tests expect a directory structure that mimics a Google Takeout archive:

```
testdata/
└── takeout/
    └── Google Photos/
        ├── Photos from 2021/
        │   ├── photo1.jpg
        │   ├── photo1.json
        │   ├── photo2.jpg
        │   └── photo2.json
        └── Albums/
            └── Vacation/
                ├── album.json
                ├── vacation1.jpg
                └── vacation1.json
```

You can create this structure manually or use a small subset of an actual Google Takeout archive.

## Integrating Tests into Your Workflow

### CI/CD Pipeline

Add testing to your GitHub Actions workflow:

```yaml
name: Tests

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      minio:
        image: minio/minio
        ports:
          - 9000:9000
        env:
          MINIO_ROOT_USER: minioadmin
          MINIO_ROOT_PASSWORD: minioadmin
        options: >-
          --name=minio-server
          --health-cmd "curl -f http://localhost:9000/minio/health/live"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 3
          --entrypoint sh
        volumes:
          - /tmp:/data
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.20'
          
      - name: Create test bucket
        run: |
          docker exec minio-server mkdir -p /data/test-bucket
          
      - name: Run unit tests
        run: go test -v ./internal/... ./pkg/...
      
      - name: Run integration tests
        run: |
          INTEGRATION_TEST=true \
          TEST_S3_ENDPOINT=localhost:9000 \
          TEST_S3_BUCKET=test-bucket \
          go test -v ./tests
          
      - name: Generate coverage report
        run: |
          go test -coverprofile=coverage.out ./...
          go tool cover -html=coverage.out -o coverage.html
          
      - name: Upload coverage report
        uses: actions/upload-artifact@v3
        with:
          name: coverage-report
          path: coverage.html
```

### Pre-commit Hook

Set up a pre-commit hook to run tests before committing:

```bash
#!/bin/bash
# .git/hooks/pre-commit or .githooks/pre-commit

# Run unit tests
go test ./internal/... ./pkg/...

# Exit with error code if tests fail
if [ $? -ne 0 ]; then
  echo "❌ Unit tests failed. Please fix before committing."
  exit 1
fi

echo "✅ All tests passed!"
exit 0
```

Make the hook executable:

```bash
chmod +x .git/hooks/pre-commit
```

Or configure Git to use a .githooks directory:

```bash
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit
```

### Code Coverage

Generate and view test coverage:

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out

# View coverage summary in terminal
go tool cover -func=coverage.out
```

## Troubleshooting

### Common Issues

1. **Integration tests fail to connect to S3**:
   - Check if your S3 service is running
   - Verify credentials and endpoint are correct
   - Ensure the test bucket exists

2. **"No such file or directory" errors**:
   - Ensure your `TEST_TAKEOUT_PATH` points to a valid directory
   - Create the test data structure as described above

3. **Tests timeout**:
   - Increase the test timeout: `go test -timeout 10m ./tests`

For more help, please open an issue on the GitHub repository. 