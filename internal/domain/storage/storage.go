package storage

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrFileNotFound         = errors.New("file not found")
	ErrInvalidFileType      = errors.New("invalid file type")
	ErrFileTooLarge         = errors.New("file too large")
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
)

// File represents a stored file
type File struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	UserID       string                 `json:"userId" gorm:"not null;index"`
	TeamID       string                 `json:"teamId" gorm:"index"`
	Name         string                 `json:"name" gorm:"not null"`
	OriginalName string                 `json:"originalName" gorm:"not null"`
	MimeType     string                 `json:"mimeType" gorm:"not null"`
	Size         int64                  `json:"size" gorm:"not null"`
	Path         string                 `json:"path" gorm:"not null"`
	Bucket       string                 `json:"bucket" gorm:"not null"`
	StorageType  string                 `json:"storageType" gorm:"default:'local'"`
	Checksum     string                 `json:"checksum"`
	Metadata     map[string]interface{} `json:"metadata" gorm:"serializer:json"`
	IsPublic     bool                   `json:"isPublic" gorm:"default:false"`
	ExpiresAt    *time.Time             `json:"expiresAt"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

// FileAccessLog represents file access history
type FileAccessLog struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	FileID    string    `json:"fileId" gorm:"not null;index"`
	UserID    string    `json:"userId" gorm:"index"`
	Action    string    `json:"action" gorm:"not null"`
	IPAddress string    `json:"ipAddress"`
	UserAgent string    `json:"userAgent"`
	CreatedAt time.Time `json:"createdAt"`
}

// Storage types
const (
	StorageTypeLocal = "local"
	StorageTypeS3    = "s3"
	StorageTypeGCS   = "gcs"
	StorageTypeAzure = "azure"
)

// Access actions
const (
	ActionUpload   = "upload"
	ActionDownload = "download"
	ActionDelete   = "delete"
	ActionView     = "view"
)

// Allowed MIME types
var AllowedMimeTypes = map[string]bool{
	"image/jpeg":       true,
	"image/png":        true,
	"image/gif":        true,
	"image/webp":       true,
	"application/pdf":  true,
	"application/json": true,
	"text/plain":       true,
	"text/csv":         true,
	"application/zip":  true,
}

// MaxFileSize is the maximum allowed file size (50MB)
const MaxFileSize = 50 * 1024 * 1024

// NewFile creates a new file record
func NewFile(userID, originalName, mimeType string, size int64) *File {
	ext := filepath.Ext(originalName)
	name := uuid.New().String() + ext

	return &File{
		ID:           uuid.New().String(),
		UserID:       userID,
		Name:         name,
		OriginalName: originalName,
		MimeType:     mimeType,
		Size:         size,
		StorageType:  StorageTypeLocal,
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// Validate validates the file
func (f *File) Validate() error {
	if f.UserID == "" {
		return errors.New("user ID is required")
	}
	if f.OriginalName == "" {
		return errors.New("file name is required")
	}
	if f.Size > MaxFileSize {
		return ErrFileTooLarge
	}
	if !AllowedMimeTypes[f.MimeType] {
		return ErrInvalidFileType
	}
	return nil
}

// IsExpired checks if the file has expired
func (f *File) IsExpired() bool {
	if f.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*f.ExpiresAt)
}

// GetExtension returns the file extension
func (f *File) GetExtension() string {
	return strings.ToLower(filepath.Ext(f.OriginalName))
}

// IsImage checks if the file is an image
func (f *File) IsImage() bool {
	return strings.HasPrefix(f.MimeType, "image/")
}

// StorageQuota represents user storage quota
type StorageQuota struct {
	UserID     string `json:"userId"`
	UsedBytes  int64  `json:"usedBytes"`
	TotalBytes int64  `json:"totalBytes"`
	FileCount  int    `json:"fileCount"`
	MaxFiles   int    `json:"maxFiles"`
}

// UploadRequest represents a file upload request
type UploadRequest struct {
	UserID    string
	TeamID    string
	FileName  string
	MimeType  string
	Size      int64
	Content   []byte
	IsPublic  bool
	ExpiresIn *time.Duration
	Metadata  map[string]interface{}
}

// DownloadResponse represents a file download response
type DownloadResponse struct {
	File        *File
	Content     []byte
	ContentType string
	URL         string
}
