package services

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
)

const maxAttachmentSize = 10 * 1024 * 1024 // 10 MB

var allowedMIMEPrefixes = []string{
	"image/",
	"text/plain",
	"text/csv",
	"application/pdf",
	"application/json",
	"application/zip",
}

// AttachmentService manages file uploads attached to incidents.
type AttachmentService interface {
	Upload(incidentID uuid.UUID, fileName, uploadedBy string, r io.Reader) (*models.IncidentAttachment, error)
	List(incidentID uuid.UUID) ([]models.IncidentAttachment, error)
	Download(id uuid.UUID) (*models.IncidentAttachment, []byte, error)
	Delete(id uuid.UUID) error
}

type attachmentService struct {
	repo repository.AttachmentRepository
}

func NewAttachmentService(repo repository.AttachmentRepository) AttachmentService {
	return &attachmentService{repo: repo}
}

func (s *attachmentService) Upload(incidentID uuid.UUID, fileName, uploadedBy string, r io.Reader) (*models.IncidentAttachment, error) {
	buf, err := io.ReadAll(io.LimitReader(r, maxAttachmentSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading upload: %w", err)
	}
	if int64(len(buf)) > maxAttachmentSize {
		return nil, fmt.Errorf("file exceeds maximum allowed size of 10 MB")
	}

	mimeType := http.DetectContentType(buf)
	mimeType = strings.TrimSpace(strings.Split(mimeType, ";")[0])

	if !isMIMEAllowed(mimeType) {
		return nil, fmt.Errorf("file type %q is not allowed", mimeType)
	}

	fileName = filepath.Base(strings.ReplaceAll(fileName, "\x00", ""))
	if fileName == "" || fileName == "." {
		fileName = "attachment-" + time.Now().UTC().Format("2006-01-02-15-04-05")
	}

	att := &models.IncidentAttachment{
		IncidentID: incidentID,
		FileName:   fileName,
		FileSize:   int64(len(buf)),
		MimeType:   mimeType,
		UploadedBy: uploadedBy,
	}
	if err := s.repo.Create(att, buf); err != nil {
		return nil, err
	}
	return att, nil
}

func (s *attachmentService) List(incidentID uuid.UUID) ([]models.IncidentAttachment, error) {
	return s.repo.ListByIncident(incidentID)
}

func (s *attachmentService) Download(id uuid.UUID) (*models.IncidentAttachment, []byte, error) {
	return s.repo.GetWithData(id)
}

func (s *attachmentService) Delete(id uuid.UUID) error {
	return s.repo.Delete(id)
}

func isMIMEAllowed(mimeType string) bool {
	for _, allowed := range allowedMIMEPrefixes {
		if strings.HasSuffix(allowed, "/") {
			if strings.HasPrefix(mimeType, allowed) {
				return true
			}
		} else if mimeType == allowed {
			return true
		}
	}
	return false
}

// ClipboardFileName returns a timestamped name for clipboard-pasted images.
func ClipboardFileName() string {
	return "screenshot-" + time.Now().UTC().Format("2006-01-02-15-04-05") + ".png"
}
