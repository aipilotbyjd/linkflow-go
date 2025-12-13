package archival

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/database"
	"gorm.io/gorm"
)

// Storage interface for archival storage backends
type Storage interface {
	// Upload uploads data to storage
	Upload(ctx context.Context, key string, data []byte) error
	
	// Download downloads data from storage
	Download(ctx context.Context, key string) ([]byte, error)
	
	// Delete deletes data from storage
	Delete(ctx context.Context, key string) error
	
	// List lists objects with prefix
	List(ctx context.Context, prefix string) ([]string, error)
	
	// Exists checks if object exists
	Exists(ctx context.Context, key string) (bool, error)
}

// S3Storage implements Storage for AWS S3
type S3Storage struct {
	client *s3.S3
	bucket string
}

// NewS3Storage creates a new S3 storage
func NewS3Storage(client *s3.S3, bucket string) *S3Storage {
	return &S3Storage{
		client: client,
		bucket: bucket,
	}
}

// Upload uploads data to S3
func (s *S3Storage) Upload(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

// Download downloads data from S3
func (s *S3Storage) Download(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	
	return io.ReadAll(result.Body)
}

// Delete deletes object from S3
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// List lists objects in S3 with prefix
func (s *S3Storage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	
	err := s.client.ListObjectsV2PagesWithContext(ctx,
		&s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
			Prefix: aws.String(prefix),
		},
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				keys = append(keys, *obj.Key)
			}
			return !lastPage
		})
	
	return keys, err
}

// Exists checks if object exists in S3
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		// Check if it's a not found error
		return false, nil
	}
	
	return true, nil
}

// Compressor interface for data compression
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// GzipCompressor implements Compressor using gzip
type GzipCompressor struct {
	level int
}

// NewGzipCompressor creates a new gzip compressor
func NewGzipCompressor() *GzipCompressor {
	return &GzipCompressor{
		level: gzip.BestCompression,
	}
}

// Compress compresses data using gzip
func (c *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, err
	}
	
	if _, err := gz.Write(data); err != nil {
		gz.Close()
		return nil, err
	}
	
	if err := gz.Close(); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

// Decompress decompresses gzip data
func (c *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	
	return io.ReadAll(gz)
}

// Archiver handles data archival
type Archiver struct {
	db          *database.DB
	storage     Storage
	compressor  Compressor
	retentionDays int
	batchSize   int
}

// NewArchiver creates a new archiver
func NewArchiver(db *database.DB, storage Storage, compressor Compressor, retentionDays int) *Archiver {
	return &Archiver{
		db:            db,
		storage:       storage,
		compressor:    compressor,
		retentionDays: retentionDays,
		batchSize:     1000,
	}
}

// ArchiveExecutions archives old execution data
func (a *Archiver) ArchiveExecutions(ctx context.Context, before time.Time) error {
	// Process in batches to avoid memory issues
	offset := 0
	
	for {
		// Query batch of executions
		var executions []workflow.WorkflowExecution
		err := a.db.WithContext(ctx).
			Where("created_at < ?", before).
			Order("created_at ASC").
			Limit(a.batchSize).
			Offset(offset).
			Preload("NodeExecutions").
			Find(&executions).Error
		
		if err != nil {
			return fmt.Errorf("failed to query executions: %w", err)
		}
		
		if len(executions) == 0 {
			break // No more executions to archive
		}
		
		// Archive batch
		if err := a.archiveBatch(ctx, executions); err != nil {
			return fmt.Errorf("failed to archive batch: %w", err)
		}
		
		// Delete archived executions
		if err := a.deleteArchivedExecutions(ctx, executions); err != nil {
			return fmt.Errorf("failed to delete archived executions: %w", err)
		}
		
		offset += len(executions)
	}
	
	return nil
}

// archiveBatch archives a batch of executions
func (a *Archiver) archiveBatch(ctx context.Context, executions []workflow.WorkflowExecution) error {
	// Group by date for better organization
	byDate := make(map[string][]workflow.WorkflowExecution)
	
	for _, exec := range executions {
		date := exec.CreatedAt.Format("2006-01-02")
		byDate[date] = append(byDate[date], exec)
	}
	
	// Archive each date group
	for date, execs := range byDate {
		archive := &ExecutionArchive{
			ID:         uuid.New().String(),
			Date:       date,
			Count:      len(execs),
			Executions: execs,
			CreatedAt:  time.Now(),
		}
		
		// Serialize archive
		data, err := json.Marshal(archive)
		if err != nil {
			return fmt.Errorf("failed to serialize archive: %w", err)
		}
		
		// Compress data
		compressed, err := a.compressor.Compress(data)
		if err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		
		// Calculate compression ratio
		compressionRatio := float64(len(data)-len(compressed)) / float64(len(data)) * 100
		
		// Upload to storage
		key := fmt.Sprintf("archive/executions/%s/%s.gz", date, archive.ID)
		if err := a.storage.Upload(ctx, key, compressed); err != nil {
			return fmt.Errorf("failed to upload archive: %w", err)
		}
		
		// Record archive metadata
		metadata := &ArchiveMetadata{
			ID:               archive.ID,
			Type:             "executions",
			Date:             date,
			RecordCount:      archive.Count,
			OriginalSize:     int64(len(data)),
			CompressedSize:   int64(len(compressed)),
			CompressionRatio: compressionRatio,
			StorageKey:       key,
			CreatedAt:        time.Now(),
		}
		
		if err := a.db.WithContext(ctx).Create(metadata).Error; err != nil {
			return fmt.Errorf("failed to save archive metadata: %w", err)
		}
	}
	
	return nil
}

// deleteArchivedExecutions deletes executions that have been archived
func (a *Archiver) deleteArchivedExecutions(ctx context.Context, executions []workflow.WorkflowExecution) error {
	ids := make([]string, len(executions))
	for i, exec := range executions {
		ids[i] = exec.ID
	}
	
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete node executions
		if err := tx.Where("execution_id IN ?", ids).
			Delete(&workflow.NodeExecution{}).Error; err != nil {
			return err
		}
		
		// Delete workflow executions
		if err := tx.Where("id IN ?", ids).
			Delete(&workflow.WorkflowExecution{}).Error; err != nil {
			return err
		}
		
		return nil
	})
}

// RestoreExecution restores an archived execution
func (a *Archiver) RestoreExecution(ctx context.Context, executionID string, date string) (*workflow.WorkflowExecution, error) {
	// Find archive containing the execution
	var metadata ArchiveMetadata
	err := a.db.WithContext(ctx).
		Where("type = ? AND date = ?", "executions", date).
		Where("storage_key LIKE ?", fmt.Sprintf("%%/%s.gz", date)).
		First(&metadata).Error
	
	if err != nil {
		return nil, fmt.Errorf("archive metadata not found: %w", err)
	}
	
	// Download from storage
	compressed, err := a.storage.Download(ctx, metadata.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to download archive: %w", err)
	}
	
	// Decompress
	data, err := a.compressor.Decompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress archive: %w", err)
	}
	
	// Deserialize archive
	var archive ExecutionArchive
	if err := json.Unmarshal(data, &archive); err != nil {
		return nil, fmt.Errorf("failed to deserialize archive: %w", err)
	}
	
	// Find specific execution
	for _, exec := range archive.Executions {
		if exec.ID == executionID {
			return &exec, nil
		}
	}
	
	return nil, fmt.Errorf("execution not found in archive")
}

// CleanupOldArchives removes archives older than retention period
func (a *Archiver) CleanupOldArchives(ctx context.Context) error {
	cutoffDate := time.Now().AddDate(0, 0, -a.retentionDays*2) // Keep archives for 2x retention period
	
	// Find old archives
	var oldArchives []ArchiveMetadata
	err := a.db.WithContext(ctx).
		Where("created_at < ?", cutoffDate).
		Find(&oldArchives).Error
	
	if err != nil {
		return err
	}
	
	// Delete from storage and database
	for _, archive := range oldArchives {
		// Delete from storage
		if err := a.storage.Delete(ctx, archive.StorageKey); err != nil {
			// Log error but continue
			_ = err
		}
		
		// Delete metadata
		if err := a.db.WithContext(ctx).Delete(&archive).Error; err != nil {
			// Log error but continue
			_ = err
		}
	}
	
	return nil
}

// GetArchiveStats returns statistics about archived data
func (a *Archiver) GetArchiveStats(ctx context.Context) (*ArchiveStats, error) {
	stats := &ArchiveStats{}
	
	// Total archives
	a.db.Model(&ArchiveMetadata{}).Count(&stats.TotalArchives)
	
	// Total records
	a.db.Model(&ArchiveMetadata{}).
		Select("SUM(record_count)").
		Scan(&stats.TotalRecords)
	
	// Storage size
	a.db.Model(&ArchiveMetadata{}).
		Select("SUM(compressed_size)").
		Scan(&stats.TotalStorageSize)
	
	// Original size
	var originalSize int64
	a.db.Model(&ArchiveMetadata{}).
		Select("SUM(original_size)").
		Scan(&originalSize)
	
	// Average compression ratio
	if originalSize > 0 {
		stats.AverageCompressionRatio = float64(originalSize-stats.TotalStorageSize) / 
			float64(originalSize) * 100
	}
	
	// Archives by type
	type TypeCount struct {
		Type  string
		Count int64
	}
	
	var typeCounts []TypeCount
	a.db.Model(&ArchiveMetadata{}).
		Select("type, COUNT(*) as count").
		Group("type").
		Scan(&typeCounts)
	
	stats.ArchivesByType = make(map[string]int64)
	for _, tc := range typeCounts {
		stats.ArchivesByType[tc.Type] = tc.Count
	}
	
	// Recent archives
	a.db.Model(&ArchiveMetadata{}).
		Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).
		Count(&stats.RecentArchives)
	
	return stats, nil
}

// Types for archival

// ExecutionArchive represents archived execution data
type ExecutionArchive struct {
	ID         string                       `json:"id"`
	Date       string                       `json:"date"`
	Count      int                          `json:"count"`
	Executions []workflow.WorkflowExecution `json:"executions"`
	CreatedAt  time.Time                    `json:"createdAt"`
}

// ArchiveMetadata tracks archived data
type ArchiveMetadata struct {
	ID               string    `gorm:"primaryKey"`
	Type             string    `gorm:"not null;index"`
	Date             string    `gorm:"not null;index"`
	RecordCount      int       `gorm:"not null"`
	OriginalSize     int64     `gorm:"not null"`
	CompressedSize   int64     `gorm:"not null"`
	CompressionRatio float64   `gorm:"not null"`
	StorageKey       string    `gorm:"not null;unique"`
	CreatedAt        time.Time `gorm:"not null;index"`
}

// ArchiveStats contains archive statistics
type ArchiveStats struct {
	TotalArchives           int64
	TotalRecords            int64
	TotalStorageSize        int64
	AverageCompressionRatio float64
	ArchivesByType          map[string]int64
	RecentArchives          int64
}
