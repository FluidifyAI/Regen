package commands

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/config"
	"github.com/FluidifyAI/Regen/backend/internal/importer"
	"github.com/FluidifyAI/Regen/backend/internal/integrations/pagerduty"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import data from external services",
	}
	cmd.AddCommand(newImportPagerdutyCmd())
	return cmd
}

func newImportPagerdutyCmd() *cobra.Command {
	var (
		apiKey     string
		force      bool
		reportFile string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "pagerduty",
		Short: "Import schedules and escalation policies from PagerDuty",
		Long: `Reads schedules and escalation policies from the PagerDuty REST API v2
and creates matching records in the Fluidify Regen database.

The DATABASE_URL environment variable must be set (same as for 'serve').

On conflict (same name already exists) the import is skipped unless --force is used.
A JSON report is written to --report (default: oi-import-report.json).`,
		Example: `  regen import pagerduty --api-key=u+xxxx
  regen import pagerduty --api-key=u+xxxx --force
  regen import pagerduty --api-key=u+xxxx --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportPagerduty(apiKey, force, reportFile, dryRun)
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "PagerDuty API key (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite records with conflicting names")
	cmd.Flags().StringVar(&reportFile, "report", "oi-import-report.json", "Path to write the JSON import report")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Fetch and validate data without writing to the database")
	_ = cmd.MarkFlagRequired("api-key")

	return cmd
}

func runImportPagerduty(apiKey string, force bool, reportFile string, dryRun bool) error {
	report := &importer.ImportReport{ImportedAt: time.Now()}

	// ── 1. Validate PagerDuty API key ───────────────────────────────────────
	pdClient := pagerduty.NewClient(apiKey)
	slog.Info("validating PagerDuty API key…")
	if err := pdClient.ValidateAPIKey(); err != nil {
		return fmt.Errorf("PagerDuty API key validation failed: %w", err)
	}
	slog.Info("API key valid")

	// ── 2. Fetch users (email → name mapping) ───────────────────────────────
	// FetchUsers already returns a map[email]name — no further processing needed.
	slog.Info("fetching PagerDuty users…")
	emailToName, err := pdClient.FetchUsers()
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}
	slog.Info("users fetched", "count", len(emailToName))

	// ── 3. Fetch schedules ──────────────────────────────────────────────────
	slog.Info("fetching PagerDuty schedules…")
	pdSchedules, err := pdClient.FetchSchedules()
	if err != nil {
		return fmt.Errorf("fetching schedules: %w", err)
	}
	slog.Info("schedules fetched", "count", len(pdSchedules))

	// Fetch detailed layer info for each schedule (layers only appear in detail view)
	scheduleDetails := make([]pagerduty.PDScheduleDetail, 0, len(pdSchedules))
	for _, s := range pdSchedules {
		detail, err := pdClient.FetchScheduleDetail(s.ID)
		if err != nil {
			slog.Warn("failed to fetch schedule detail; skipping", "schedule_id", s.ID, "name", s.Name, "err", err)
			continue
		}
		scheduleDetails = append(scheduleDetails, *detail)
	}

	// ── 4. Fetch escalation policies ────────────────────────────────────────
	slog.Info("fetching PagerDuty escalation policies…")
	pdPolicies, err := pdClient.FetchEscalationPolicies()
	if err != nil {
		return fmt.Errorf("fetching escalation policies: %w", err)
	}
	slog.Info("escalation policies fetched", "count", len(pdPolicies))

	policyDetails := make([]pagerduty.PDEscalationPolicyDetail, 0, len(pdPolicies))
	for _, p := range pdPolicies {
		detail, err := pdClient.FetchEscalationPolicyDetail(p.ID)
		if err != nil {
			slog.Warn("failed to fetch policy detail; skipping", "policy_id", p.ID, "name", p.Name, "err", err)
			continue
		}
		policyDetails = append(policyDetails, *detail)
	}

	// ── 5. Dry-run: print summary and exit ──────────────────────────────────
	if dryRun {
		fmt.Printf("Dry run — no database writes.\n")
		fmt.Printf("  Schedules found:          %d\n", len(scheduleDetails))
		fmt.Printf("  Escalation policies found: %d\n", len(policyDetails))
		fmt.Printf("  Users fetched:            %d\n", len(emailToName))
		return nil
	}

	// ── 6. Open database connection ─────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	slog.Info("database connected")

	// ── 7. Import schedules ─────────────────────────────────────────────────
	scheduleRepo := repository.NewScheduleRepository(db)
	slog.Info("importing schedules…")
	if err := importer.ImportSchedules(scheduleRepo, scheduleDetails, emailToName, force, report); err != nil {
		return fmt.Errorf("importing schedules: %w", err)
	}

	// Build schedule name → new OI UUID map for policy tier linkage
	scheduleNameToID, err := buildScheduleNameMap(scheduleRepo)
	if err != nil {
		return fmt.Errorf("building schedule name map: %w", err)
	}

	// ── 8. Import escalation policies ───────────────────────────────────────
	escalationPolicyRepo := repository.NewEscalationPolicyRepository(db)
	slog.Info("importing escalation policies…")
	if err := importer.ImportPolicies(escalationPolicyRepo, policyDetails, scheduleNameToID, emailToName, force, report); err != nil {
		return fmt.Errorf("importing escalation policies: %w", err)
	}

	// ── 9. Write report ─────────────────────────────────────────────────────
	report.PrintSummary()
	if err := report.WriteToFile(reportFile); err != nil {
		slog.Warn("failed to write report file", "path", reportFile, "err", err)
	} else {
		fmt.Printf("\nReport written to %s\n", reportFile)
	}

	if len(report.Errors) > 0 {
		os.Exit(1)
	}
	return nil
}

// buildScheduleNameMap returns a map of schedule name → UUID for all schedules
// currently in the database, so policy tiers can reference them by name.
func buildScheduleNameMap(repo repository.ScheduleRepository) (map[string]uuid.UUID, error) {
	schedules, err := repo.GetAll()
	if err != nil {
		return nil, err
	}
	m := make(map[string]uuid.UUID, len(schedules))
	for _, s := range schedules {
		m[s.Name] = s.ID
	}
	return m, nil
}
