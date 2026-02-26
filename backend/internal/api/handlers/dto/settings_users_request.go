package dto

type CreateUserRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Name     string `json:"name"     binding:"required,min=1"`
	Role     string `json:"role"     binding:"required,oneof=admin member viewer"`
	Password string `json:"password" binding:"required,min=8"`
}

type UpdateUserRequest struct {
	Name     string `json:"name"`
	Role     string `json:"role"     binding:"omitempty,oneof=admin member viewer"`
	Password string `json:"password" binding:"omitempty,min=8"`
}
