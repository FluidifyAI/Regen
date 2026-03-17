package dto

import "github.com/fluidify/regen/internal/repository"

// PaginationParams holds pagination query parameters
type PaginationParams struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"limit" binding:"omitempty,min=1,max=250"`
	Offset   int `form:"offset" binding:"omitempty,min=0"`
}

// Normalize applies default values and constraints to pagination parameters
func (p *PaginationParams) Normalize() {
	if p.Page == 0 {
		p.Page = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 50 // Default
	}
	if p.PageSize > 250 {
		p.PageSize = 250 // Max
	}
}

// ToRepository converts API pagination params to repository pagination
func (p *PaginationParams) ToRepository() repository.Pagination {
	return repository.Pagination{
		Page:     p.Page,
		PageSize: p.PageSize,
	}
}

// PaginatedResponse is the standard paginated response format
type PaginatedResponse struct {
	Data   interface{} `json:"data"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}
