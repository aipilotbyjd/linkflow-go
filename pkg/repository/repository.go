package repository

import (
	"context"
)

// Repository is a generic repository interface
type Repository[T any] interface {
	Create(ctx context.Context, entity *T) error
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*T, error)
	FindAll(ctx context.Context) ([]*T, error)
}

// PaginatedResult represents a paginated query result
type PaginatedResult[T any] struct {
	Items      []*T  `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	TotalPages int   `json:"totalPages"`
}

// Pagination represents pagination parameters
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// Filter represents a query filter
type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // eq, ne, gt, gte, lt, lte, like, in
	Value    interface{} `json:"value"`
}

// Sort represents a sort order
type Sort struct {
	Field string `json:"field"`
	Order string `json:"order"` // asc, desc
}

// QueryOptions represents query options
type QueryOptions struct {
	Pagination *Pagination `json:"pagination"`
	Filters    []Filter    `json:"filters"`
	Sorts      []Sort      `json:"sorts"`
}

// NewPagination creates pagination with defaults
func NewPagination(page, pageSize int) *Pagination {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return &Pagination{
		Page:     page,
		PageSize: pageSize,
	}
}

// Offset returns the offset for the pagination
func (p *Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// NewPaginatedResult creates a new paginated result
func NewPaginatedResult[T any](items []*T, total int64, pagination *Pagination) *PaginatedResult[T] {
	totalPages := int(total) / pagination.PageSize
	if int(total)%pagination.PageSize > 0 {
		totalPages++
	}

	return &PaginatedResult[T]{
		Items:      items,
		Total:      total,
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages,
	}
}
