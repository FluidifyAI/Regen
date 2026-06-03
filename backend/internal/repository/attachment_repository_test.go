package repository_test

import (
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachmentRepository_CreateAndList(t *testing.T) {
	db := setupTestDB(t)
	incidentRepo := repository.NewIncidentRepository(db)
	repo := repository.NewAttachmentRepository(db)

	inc := makeTestIncident()
	require.NoError(t, incidentRepo.Create(inc))

	att := &models.IncidentAttachment{
		IncidentID: inc.ID,
		FileName:   "screenshot.png",
		FileSize:   1024,
		MimeType:   "image/png",
		UploadedBy: "alice",
	}
	data := []byte("fake-image-bytes")
	require.NoError(t, repo.Create(att, data))
	assert.NotEqual(t, uuid.Nil, att.ID)

	list, err := repo.ListByIncident(inc.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "screenshot.png", list[0].FileName)
}

func TestAttachmentRepository_GetWithData(t *testing.T) {
	db := setupTestDB(t)
	incidentRepo := repository.NewIncidentRepository(db)
	repo := repository.NewAttachmentRepository(db)

	inc := makeTestIncident()
	require.NoError(t, incidentRepo.Create(inc))

	att := &models.IncidentAttachment{
		IncidentID: inc.ID,
		FileName:   "log.txt",
		FileSize:   512,
		MimeType:   "text/plain",
		UploadedBy: "bob",
	}
	content := []byte("error: connection refused")
	require.NoError(t, repo.Create(att, content))

	fetched, fetchedData, err := repo.GetWithData(att.ID)
	require.NoError(t, err)
	assert.Equal(t, "log.txt", fetched.FileName)
	assert.Equal(t, content, fetchedData)
}

func TestAttachmentRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	incidentRepo := repository.NewIncidentRepository(db)
	repo := repository.NewAttachmentRepository(db)

	inc := makeTestIncident()
	require.NoError(t, incidentRepo.Create(inc))

	att := &models.IncidentAttachment{
		IncidentID: inc.ID,
		FileName:   "delete-me.pdf",
		FileSize:   2048,
		MimeType:   "application/pdf",
		UploadedBy: "carol",
	}
	require.NoError(t, repo.Create(att, []byte("pdf-content")))

	require.NoError(t, repo.Delete(att.ID))

	list, err := repo.ListByIncident(inc.ID)
	require.NoError(t, err)
	assert.Empty(t, list)

	// Also verify the data row was cascade-deleted
	_, _, dataErr := repo.GetWithData(att.ID)
	require.Error(t, dataErr)
	_, isNotFound := dataErr.(*repository.NotFoundError)
	assert.True(t, isNotFound, "expected NotFoundError after cascade delete of data row")
}

func TestAttachmentRepository_GetWithData_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewAttachmentRepository(db)

	_, _, err := repo.GetWithData(uuid.New())
	require.Error(t, err)
	_, ok := err.(*repository.NotFoundError)
	assert.True(t, ok)
}
