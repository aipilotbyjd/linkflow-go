package search

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowIndex represents a searchable workflow index
type WorkflowIndex struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	WorkflowID   string    `json:"workflowId" gorm:"uniqueIndex;not null"`
	UserID       string    `json:"userId" gorm:"not null;index"`
	Name         string    `json:"name" gorm:"not null"`
	Description  string    `json:"description"`
	Tags         []string  `json:"tags" gorm:"serializer:json"`
	NodeTypes    []string  `json:"nodeTypes" gorm:"serializer:json"`
	SearchVector string    `json:"searchVector" gorm:"type:tsvector"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// SearchHistory represents user search history
type SearchHistory struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	UserID       string                 `json:"userId" gorm:"not null;index"`
	Query        string                 `json:"query" gorm:"not null"`
	Filters      map[string]interface{} `json:"filters" gorm:"serializer:json"`
	ResultsCount int                    `json:"resultsCount" gorm:"default:0"`
	CreatedAt    time.Time              `json:"createdAt"`
}

// SearchQuery represents a search request
type SearchQuery struct {
	Query      string           `json:"query"`
	Filters    SearchFilters    `json:"filters"`
	Sort       SearchSort       `json:"sort"`
	Pagination SearchPagination `json:"pagination"`
}

// SearchFilters represents search filters
type SearchFilters struct {
	UserID    string     `json:"userId"`
	TeamID    string     `json:"teamId"`
	Status    string     `json:"status"`
	Tags      []string   `json:"tags"`
	NodeTypes []string   `json:"nodeTypes"`
	DateFrom  *time.Time `json:"dateFrom"`
	DateTo    *time.Time `json:"dateTo"`
	IsActive  *bool      `json:"isActive"`
}

// SearchSort represents sort options
type SearchSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"` // asc, desc
}

// SearchPagination represents pagination options
type SearchPagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID          string    `json:"id"`
	WorkflowID  string    `json:"workflowId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Status      string    `json:"status"`
	IsActive    bool      `json:"isActive"`
	Score       float64   `json:"score"`
	Highlights  []string  `json:"highlights"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SearchResponse represents a search response
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalPages int            `json:"totalPages"`
	Query      string         `json:"query"`
	Took       int64          `json:"took"` // milliseconds
}

// Suggestion represents a search suggestion
type Suggestion struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	Type  string  `json:"type"` // workflow, tag, node
}

// NewWorkflowIndex creates a new workflow index
func NewWorkflowIndex(workflowID, userID, name, description string) *WorkflowIndex {
	return &WorkflowIndex{
		ID:          uuid.New().String(),
		WorkflowID:  workflowID,
		UserID:      userID,
		Name:        name,
		Description: description,
		Tags:        []string{},
		NodeTypes:   []string{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// NewSearchHistory creates a new search history entry
func NewSearchHistory(userID, query string, resultsCount int) *SearchHistory {
	return &SearchHistory{
		ID:           uuid.New().String(),
		UserID:       userID,
		Query:        query,
		Filters:      make(map[string]interface{}),
		ResultsCount: resultsCount,
		CreatedAt:    time.Now(),
	}
}

// DefaultPagination returns default pagination settings
func DefaultPagination() SearchPagination {
	return SearchPagination{
		Page:  1,
		Limit: 20,
	}
}

// DefaultSort returns default sort settings
func DefaultSort() SearchSort {
	return SearchSort{
		Field:     "updatedAt",
		Direction: "desc",
	}
}
