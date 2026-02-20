package dto

// CreatePostMortemTemplateRequest is the body for POST /api/v1/post-mortem-templates.
type CreatePostMortemTemplateRequest struct {
	Name        string   `json:"name"        binding:"required,min=1,max=100"`
	Description string   `json:"description" binding:"max=1000"`
	Sections    []string `json:"sections"    binding:"required,min=1,dive,min=1,max=100"`
}

// UpdatePostMortemTemplateRequest is the body for PATCH /api/v1/post-mortem-templates/:id.
type UpdatePostMortemTemplateRequest struct {
	Name        *string  `json:"name"        binding:"omitempty,min=1,max=100"`
	Description *string  `json:"description" binding:"omitempty,max=1000"`
	Sections    []string `json:"sections"    binding:"omitempty,min=1,dive,min=1,max=100"`
}
