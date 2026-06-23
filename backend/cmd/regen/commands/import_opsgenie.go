package commands

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/config"
	"github.com/FluidifyAI/Regen/backend/internal/importer"
	"github.com/FluidifyAI/Regen/backend/internal/integrations/opsgenie"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newImportOpsgenieCmd() *cobra.Command {
	var (
		apiKey     string
		region     string
		force      bool
		reportFile string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "opsgenie",
		Short: "Import schedules and escalation policies from Opsgenie",
		Long: `Reads schedules and escalation policies from the Opsgenie REST API v2
and creates matching records in the Fluidify Regen database.

The DATABASE_URL environment variable must be set (same as for 'serve').

On conflict (same name already exists) the import is skipped unless --force is used.
A JSON report is written to --report (default: oi-import-report.json).`,
		Example: `  regen import opsgenie --api-key=<key>
  regen import opsgenie --api-key=<key> --region=eu
  regen import opsgenie --api-key=<key> --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportOpsgenie(apiKey, region, force, reportFile, dryRun)
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "Opsgenie API key (required)")
	cmd.Flags().StringVar(&region, "region", "us", "Opsgenie region: us or eu")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite records with conflicting names")
	cmd.Flags().StringVar(&reportFile, "report", "oi-import-report.json", "Path to write the JSON import report")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Fetch and validate data without writing to the database")
	_ = cmd.MarkFlagRequired("api-key")

	return cmd
}

func runImportOpsgenie(apiKey, region string, force bool, reportFile string, dryRun bool) error {
	report := &importer.ImportReport{ImportedAt: time.Now()}

	ogClient := opsgenie.NewClient(apiKey, region)
	slog.Info("validating Opsgenie API key…")
	if err := ogClient.ValidateAPIKey(); err != nil {
		return fmt.Errorf("opsgenie API key validation failed: %w", err)
	}
	slog.Info("API key valid")

	slog.Info("fetching Opsgenie users…")
	emailToName, err := ogClient.FetchUsers()
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}
	slog.Info("users fetched", "count", len(emailToName))

	slog.Info("fetching Opsgenie schedules…")
	ogSchedules, err := ogClient.FetchSchedules()
	if err != nil {
		return fmt.Errorf("fetching schedules: %w", err)
	}
	scheduleDetails := make([]opsgenie.OGScheduleDetail, 0, len(ogSchedules))
	for _, s := range ogSchedules {
		rotations, err := ogClient.FetchRotations(s.ID)
		if err != nil {
			slog.Warn("failed to fetch rotations; skipping schedule", "schedule_id", s.ID, "name", s.Name, "err", err)
			continue
		}
		scheduleDetails = append(scheduleDetails, opsgenie.OGScheduleDetail{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Timezone:    s.Timezone,
			Rotations:   rotations,
		})
	}
	slog.Info("schedules fetched", "count", len(scheduleDetails))

	slog.Info("fetching Opsgenie escalation policies…")
	policies, err := ogClient.FetchEscalationPolicies()
	if err != nil {
		return fmt.Errorf("fetching escalation policies: %w", err)
	}
	slog.Info("escalation policies fetched", "count", len(policies))

	if dryRun {
		fmt.Printf("Dry run — no database writes.\n")
		fmt.Printf("  Schedules found:           %d\n", len(scheduleDetails))
		fmt.Printf("  Escalation policies found: %d\n", len(policies))
		fmt.Printf("  Users fetched:             %d\n", len(emailToName))
		return nil
	}

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

	scheduleRepo := repository.NewScheduleRepository(db)
	slog.Info("importing schedules…")
	if err := importer.ImportOpsgenieSchedules(scheduleRepo, scheduleDetails, emailToName, force, report); err != nil {
		return fmt.Errorf("importing schedules: %w", err)
	}

	scheduleNameToID, err := ogBuildCLIScheduleNameMap(scheduleRepo)
	if err != nil {
		return fmt.Errorf("building schedule name map: %w", err)
	}

	escalationPolicyRepo := repository.NewEscalationPolicyRepository(db)
	slog.Info("importing escalation policies…")
	if err := importer.ImportOpsgeniePolicies(escalationPolicyRepo, policies, scheduleNameToID, emailToName, force, report); err != nil {
		return fmt.Errorf("importing escalation policies: %w", err)
	}

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

func ogBuildCLIScheduleNameMap(repo repository.ScheduleRepository) (map[string]uuid.UUID, error) {
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
