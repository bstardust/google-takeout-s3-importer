package common

import "fmt"

type ImportError struct {
	Message string
}

func (e *ImportError) Error() string {
	return fmt.Sprintf("Import Error: %s", e.Message)
}

type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("Configuration Error: %s", e.Message)
}

type S3Error struct {
	Message string
}

func (e *S3Error) Error() string {
	return fmt.Sprintf("S3 Error: %s", e.Message)
}

func NewImportError(message string) error {
	return &ImportError{Message: message}
}

func NewConfigError(message string) error {
	return &ConfigError{Message: message}
}

func NewS3Error(message string) error {
	return &S3Error{Message: message}
}