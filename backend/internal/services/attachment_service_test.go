package services_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAttachmentRepo struct {
	atts map[uuid.UUID]*models.IncidentAttachment
	data map[uuid.UUID][]byte
}

func newStubAttachmentRepo() *stubAttachmentRepo {
	return &stubAttachmentRepo{
		atts: make(map[uuid.UUID]*models.IncidentAttachment),
		data: make(map[uuid.UUID][]byte),
	}
}

func (s *stubAttachmentRepo) Create(att *models.IncidentAttachment, data []byte) error {
	att.ID = uuid.New()
	s.atts[att.ID] = att
	s.data[att.ID] = data
	return nil
}

func (s *stubAttachmentRepo) ListByIncident(incidentID uuid.UUID) ([]models.IncidentAttachment, error) {
	var out []models.IncidentAttachment
	for _, a := range s.atts {
		if a.IncidentID == incidentID {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (s *stubAttachmentRepo) GetWithData(id uuid.UUID) (*models.IncidentAttachment, []byte, error) {
	a, ok := s.atts[id]
	if !ok {
		return nil, nil, &repository.NotFoundError{Resource: "attachment", ID: id.String()}
	}
	return a, s.data[id], nil
}

func (s *stubAttachmentRepo) Delete(id uuid.UUID) error {
	if _, ok := s.atts[id]; !ok {
		return &repository.NotFoundError{Resource: "attachment", ID: id.String()}
	}
	delete(s.atts, id)
	delete(s.data, id)
	return nil
}

func TestAttachmentService_Upload_ValidImage(t *testing.T) {
	svc := services.NewAttachmentService(newStubAttachmentRepo())
	incID := uuid.New()

	pngBytes := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 520))
	r := bytes.NewReader(pngBytes)

	att, err := svc.Upload(incID, "screenshot.png", "alice", r)
	require.NoError(t, err)
	assert.Equal(t, "screenshot.png", att.FileName)
	assert.Equal(t, "image/png", att.MimeType)
	assert.Equal(t, int64(len(pngBytes)), att.FileSize)
}

func TestAttachmentService_Upload_TooLarge(t *testing.T) {
	svc := services.NewAttachmentService(newStubAttachmentRepo())
	incID := uuid.New()

	big := bytes.NewReader(make([]byte, 10*1024*1024+1))
	_, err := svc.Upload(incID, "big.png", "alice", big)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10 MB")
}

func TestAttachmentService_Upload_DisallowedType(t *testing.T) {
	svc := services.NewAttachmentService(newStubAttachmentRepo())
	incID := uuid.New()

	exe := bytes.NewReader(append([]byte("MZ"), make([]byte, 520)...))
	_, err := svc.Upload(incID, "virus.exe", "alice", exe)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestAttachmentService_SanitizeFilename(t *testing.T) {
	svc := services.NewAttachmentService(newStubAttachmentRepo())
	incID := uuid.New()

	pngBytes := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 520))
	r := bytes.NewReader(pngBytes)

	att, err := svc.Upload(incID, "../../etc/passwd", "alice", r)
	require.NoError(t, err)
	assert.Equal(t, "passwd", att.FileName)
}
