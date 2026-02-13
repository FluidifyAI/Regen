package handlers

import "github.com/openincident/openincident/internal/repository"

// isNotFound checks if an error is a repository NotFoundError.
// Use this instead of checking gorm.ErrRecordNotFound directly — repositories
// wrap that sentinel in *repository.NotFoundError before returning it.
func isNotFound(err error) bool {
	_, ok := err.(*repository.NotFoundError)
	return ok
}
