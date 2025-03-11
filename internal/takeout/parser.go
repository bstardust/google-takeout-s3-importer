package takeout

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Media struct {
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	FileSize int64  `json:"file_size"`
}

type TakeoutParser struct {
	FilePath string
}

func NewTakeoutParser(filePath string) *TakeoutParser {
	return &TakeoutParser{FilePath: filePath}
}

func (tp *TakeoutParser) Parse() ([]Media, error) {
	file, err := os.Open(tp.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var mediaList []Media
	if err := json.Unmarshal(data, &mediaList); err != nil {
		return nil, err
	}

	return mediaList, nil
}