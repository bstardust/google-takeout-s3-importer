package models

type Media struct {
    FileName string `json:"file_name"`
    FileType string `json:"file_type"`
    FileSize int64  `json:"file_size"`
    FilePath string `json:"file_path"`
    Metadata map[string]interface{} `json:"metadata"`
}

func NewMedia(fileName, fileType string, fileSize int64, filePath string, metadata map[string]interface{}) *Media {
    return &Media{
        FileName: fileName,
        FileType: fileType,
        FileSize: fileSize,
        FilePath: filePath,
        Metadata: metadata,
    }
}