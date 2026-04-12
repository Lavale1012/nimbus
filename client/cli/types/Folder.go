package types

// ListResponse represents the server's folder listing response
type ListResponse struct {
	FolderID   *uint         `json:"folder_id"`
	FolderName string        `json:"folder_name"`
	Path       string        `json:"path"`
	Files      []FileEntry   `json:"files"`
	Folders    []FolderEntry `json:"folders"`
}

type FileEntry struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	S3Key     string `json:"s3_key"`
	CreatedAt string `json:"created_at"`
}

type FolderEntry struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}
