// Package importer converts PagerDuty data into OpenIncident entities and
// persists them to the database.
package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ImportReport is written to disk after the import completes.
type ImportReport struct {
	ImportedAt time.Time     `json:"imported_at"`
	Summary    ReportSummary `json:"summary"`
	Warnings   []string      `json:"warnings"`
	Errors     []string      `json:"errors"`
}

// ReportSummary holds counts of imported and skipped entities.
type ReportSummary struct {
	SchedulesFound    int `json:"schedules_found"`
	SchedulesImported int `json:"schedules_imported"`
	SchedulesSkipped  int `json:"schedules_skipped"`
	LayersImported    int `json:"layers_imported"`
	LayersSkipped     int `json:"layers_skipped"`
	PoliciesFound     int `json:"policies_found"`
	PoliciesImported  int `json:"policies_imported"`
	PoliciesSkipped   int `json:"policies_skipped"`
	TiersImported     int `json:"tiers_imported"`
}

// WriteToFile serialises the report as JSON and writes it to path.
func (r *ImportReport) WriteToFile(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing report to %s: %w", path, err)
	}
	return nil
}

// PrintSummary writes a human-readable summary to stdout.
func (r *ImportReport) PrintSummary() {
	fmt.Println("─── PagerDuty Import Summary ────────────────────────")
	fmt.Printf("Schedules : %d found, %d imported, %d skipped\n",
		r.Summary.SchedulesFound, r.Summary.SchedulesImported, r.Summary.SchedulesSkipped)
	fmt.Printf("  Layers  : %d imported, %d skipped\n",
		r.Summary.LayersImported, r.Summary.LayersSkipped)
	fmt.Printf("Policies  : %d found, %d imported, %d skipped\n",
		r.Summary.PoliciesFound, r.Summary.PoliciesImported, r.Summary.PoliciesSkipped)
	fmt.Printf("  Tiers   : %d imported\n", r.Summary.TiersImported)
	if len(r.Warnings) > 0 {
		fmt.Printf("\nWarnings (%d):\n", len(r.Warnings))
		for _, w := range r.Warnings {
			fmt.Printf("  ⚠  %s\n", w)
		}
	}
	if len(r.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(r.Errors))
		for _, e := range r.Errors {
			fmt.Printf("  ✗  %s\n", e)
		}
	}
	fmt.Println("─────────────────────────────────────────────────────")
}
