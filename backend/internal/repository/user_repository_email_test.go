package repository_test

import (
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/database"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_GetByEmail_CaseInsensitive(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewUserRepository(db)

	hash := "x"
	require.NoError(t, repo.CreateLocal(&models.User{
		Email:        "Dev@Example.com",
		Name:         "Dev",
		AuthSource:   "local",
		PasswordHash: &hash,
		Role:         models.UserRoleMember,
		Active:       true,
	}))

	for _, email := range []string{"dev@example.com", "DEV@EXAMPLE.COM", "Dev@Example.com"} {
		found, err := repo.GetByEmail(email)
		require.NoError(t, err, email)
		assert.Equal(t, "Dev@Example.com", found.Email)
	}

	_, err := repo.GetByEmail("other@example.com")
	var notFound *repository.NotFoundError
	assert.ErrorAs(t, err, &notFound)
}
