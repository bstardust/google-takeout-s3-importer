package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	S3Bucket          string `json:"s3_bucket"`
	S3Region          string `json:"s3_region"`
	GoogleTakeoutPath string `json:"google_takeout_path"`
}

func LoadConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}