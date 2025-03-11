package takeout

type Metadata struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    MediaType   string `json:"media_type"`
    FileSize    int64  `json:"file_size"`
    CreatedAt   string `json:"created_at"`
    UpdatedAt   string `json:"updated_at"`
}

func NewMetadata(title, description, mediaType string, fileSize int64, createdAt, updatedAt string) *Metadata {
    return &Metadata{
        Title:       title,
        Description: description,
        MediaType:   mediaType,
        FileSize:    fileSize,
        CreatedAt:   createdAt,
        UpdatedAt:   updatedAt,
    }
}