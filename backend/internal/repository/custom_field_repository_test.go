package repository_test

import (
	"encoding/json"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/database"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeField(name, key, fieldType string) *models.CustomFieldDefinition {
	opts, _ := json.Marshal([]models.DropdownOption{})
	return &models.CustomFieldDefinition{
		Name:      name,
		Key:       key,
		FieldType: fieldType,
		Options:   json.RawMessage(opts),
	}
}

func makeDropdownField(name, key string, opts []models.DropdownOption) *models.CustomFieldDefinition {
	raw, _ := json.Marshal(opts)
	return &models.CustomFieldDefinition{
		Name:      name,
		Key:       key,
		FieldType: models.FieldTypeDropdown,
		Options:   json.RawMessage(raw),
	}
}

func TestCustomFieldRepository_CRUD(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewCustomFieldRepository(db)

	// Create
	f := makeField("Affected Service", "affected_service", models.FieldTypeString)
	require.NoError(t, repo.Create(f))
	assert.NotEmpty(t, f.ID)

	// List
	fields, err := repo.List()
	require.NoError(t, err)
	assert.Len(t, fields, 1)
	assert.Equal(t, "affected_service", fields[0].Key)

	// GetByKey
	found, err := repo.GetByKey("affected_service")
	require.NoError(t, err)
	assert.Equal(t, f.ID, found.ID)

	// Update
	f.Name = "Affected Service (updated)"
	require.NoError(t, repo.Update(f))
	found, err = repo.GetByKey("affected_service")
	require.NoError(t, err)
	assert.Equal(t, "Affected Service (updated)", found.Name)

	// Delete
	require.NoError(t, repo.Delete(f.ID))
	fields, err = repo.List()
	require.NoError(t, err)
	assert.Empty(t, fields)
}

func TestCustomFieldRepository_KeyUniqueness(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewCustomFieldRepository(db)

	f1 := makeField("Service", "service", models.FieldTypeString)
	require.NoError(t, repo.Create(f1))

	f2 := makeField("Service Dupe", "service", models.FieldTypeString)
	err := repo.Create(f2)
	require.Error(t, err)
}

func TestCustomFieldRepository_Reorder(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewCustomFieldRepository(db)

	f1 := makeField("Alpha", "alpha", models.FieldTypeString)
	f2 := makeField("Beta", "beta", models.FieldTypeString)
	require.NoError(t, repo.Create(f1))
	require.NoError(t, repo.Create(f2))

	require.NoError(t, repo.Reorder([]repository.ReorderItem{
		{ID: f1.ID, Order: 5},
		{ID: f2.ID, Order: 2},
	}))

	fields, err := repo.List()
	require.NoError(t, err)
	require.Len(t, fields, 2)
	// List is ordered by display_order ASC — beta (2) first, alpha (5) second
	assert.Equal(t, "beta", fields[0].Key)
	assert.Equal(t, "alpha", fields[1].Key)
}

func TestCustomFieldRepository_CountUsage(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewCustomFieldRepository(db)

	f := makeField("Team", "team", models.FieldTypeString)
	require.NoError(t, repo.Create(f))

	// No incidents yet — usage is zero
	count, err := repo.CountUsage("team")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCustomFieldRepository_DropdownOptions(t *testing.T) {
	db := database.SetupTestDB(t)
	repo := repository.NewCustomFieldRepository(db)

	opts := []models.DropdownOption{
		{Label: "High", Value: "high"},
		{Label: "Low", Value: "low"},
	}
	f := makeDropdownField("Priority", "priority", opts)
	require.NoError(t, repo.Create(f))

	found, err := repo.GetByKey("priority")
	require.NoError(t, err)
	assert.Equal(t, models.FieldTypeDropdown, found.FieldType)

	var decoded []models.DropdownOption
	require.NoError(t, json.Unmarshal(found.Options, &decoded))
	require.Len(t, decoded, 2)
	assert.Equal(t, "High", decoded[0].Label)
}
