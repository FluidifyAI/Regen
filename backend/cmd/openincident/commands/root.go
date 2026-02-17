package commands

import "github.com/spf13/cobra"

// NewRootCmd returns the cobra root command. All subcommands are attached here.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "openincident",
		Short: "OpenIncident — open-source incident management platform",
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newImportCmd())

	return root
}

// newImportCmd stub — replaced in Task 10
func newImportCmd() *cobra.Command {
	return &cobra.Command{Use: "import", Short: "Import data from external services (not yet implemented)"}
}
