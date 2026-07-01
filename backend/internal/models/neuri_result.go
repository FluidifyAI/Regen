package models

import (
	"time"

	"github.com/google/uuid"
)

type NeuriResult struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IncidentID         uuid.UUID `gorm:"type:uuid;not null;index"                       json:"incident_id"`
	InvestigationRunID uuid.UUID `gorm:"type:uuid;not null"                             json:"investigation_run_id"`
	TopHypothesis      string    `gorm:"type:text;not null"                             json:"top_hypothesis"`
	Confidence         float64   `gorm:"not null"                                       json:"confidence"`
	Summary            string    `gorm:"type:text;not null"                             json:"summary"`
	RankedHypotheses   RawJSON   `gorm:"type:jsonb;not null;default:'[]'"               json:"ranked_hypotheses"`
	CreatedAt          time.Time `gorm:"not null;default:now()"                         json:"created_at"`
}

func (NeuriResult) TableName() string { return "neuri_investigation_results" }
