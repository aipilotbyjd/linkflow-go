package template

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTemplateNotFound = errors.New("template not found")
	ErrInvalidTemplate  = errors.New("invalid template")
)

// WorkflowTemplate represents a reusable workflow template
type WorkflowTemplate struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	Name         string                 `json:"name" gorm:"not null"`
	Description  string                 `json:"description"`
	Category     string                 `json:"category" gorm:"index"`
	Subcategory  string                 `json:"subcategory"`
	Icon         string                 `json:"icon"`
	Color        string                 `json:"color"`
	CreatorID    string                 `json:"creatorId" gorm:"index"`
	WorkflowData map[string]interface{} `json:"workflowData" gorm:"serializer:json"`
	PreviewImage string                 `json:"previewImage"`
	IsOfficial   bool                   `json:"isOfficial" gorm:"default:false"`
	IsPublic     bool                   `json:"isPublic" gorm:"default:true"`
	IsFeatured   bool                   `json:"isFeatured" gorm:"default:false"`
	Version      string                 `json:"version" gorm:"default:'1.0.0'"`
	Downloads    int                    `json:"downloads" gorm:"default:0"`
	Rating       float32                `json:"rating" gorm:"default:0"`
	RatingsCount int                    `json:"ratingsCount" gorm:"default:0"`
	Tags         []string               `json:"tags" gorm:"serializer:json"`
	Requirements []string               `json:"requirements" gorm:"serializer:json"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

// Category represents a template category
type Category struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	ParentID    string    `json:"parentId" gorm:"index"`
	SortOrder   int       `json:"sortOrder" gorm:"default:0"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Rating represents a user rating for a template
type Rating struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	TemplateID string    `json:"templateId" gorm:"not null;index"`
	UserID     string    `json:"userId" gorm:"not null;index"`
	Rating     int       `json:"rating" gorm:"not null"`
	Review     string    `json:"review"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// NewWorkflowTemplate creates a new workflow template
func NewWorkflowTemplate(name, description, category, creatorID string) *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:           uuid.New().String(),
		Name:         name,
		Description:  description,
		Category:     category,
		CreatorID:    creatorID,
		WorkflowData: make(map[string]interface{}),
		IsPublic:     true,
		Version:      "1.0.0",
		Tags:         []string{},
		Requirements: []string{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// Validate validates the template
func (t *WorkflowTemplate) Validate() error {
	if t.Name == "" {
		return errors.New("template name is required")
	}
	if t.Category == "" {
		return errors.New("template category is required")
	}
	if t.WorkflowData == nil || len(t.WorkflowData) == 0 {
		return errors.New("workflow data is required")
	}
	return nil
}

// IncrementDownloads increments the download count
func (t *WorkflowTemplate) IncrementDownloads() {
	t.Downloads++
	t.UpdatedAt = time.Now()
}

// UpdateRating updates the template rating
func (t *WorkflowTemplate) UpdateRating(newRating int) {
	totalRating := float32(t.Rating) * float32(t.RatingsCount)
	t.RatingsCount++
	t.Rating = (totalRating + float32(newRating)) / float32(t.RatingsCount)
	t.UpdatedAt = time.Now()
}
