package s3

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type Uploader struct {
	s3Client *s3.S3
	bucket   string
}

func NewUploader(bucket string) (*Uploader, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"), // Change to your desired region
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &Uploader{
		s3Client: s3.New(sess),
		bucket:   bucket,
	}, nil
}

func (u *Uploader) UploadFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}

	_, err = u.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(filepath.Base(filePath)),
		Body:   file,
		ACL:    aws.String("public-read"), // Change as needed
		ContentType: aws.String(http.DetectContentType(make([]byte, 512))),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file %s: %w", filePath, err)
	}

	log.Printf("Successfully uploaded %s to %s\n", fileInfo.Name(), u.bucket)
	return nil
}

func (u *Uploader) UploadDirectory(dirPath string) error {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	for _, file := range files {
		if !file.IsDir() {
			filePath := filepath.Join(dirPath, file.Name())
			if err := u.UploadFile(filePath); err != nil {
				log.Printf("Error uploading file %s: %v\n", filePath, err)
			}
		}
	}
	return nil
}