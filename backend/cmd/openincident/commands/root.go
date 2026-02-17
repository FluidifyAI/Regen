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

// newServeCmd stub — replaced in Task 2
func newServeCmd() *cobra.Command {
	return &cobra.Command{Use: "serve", Short: "Start the OpenIncident HTTP server (not yet implemented)"}
}

// newImportCmd stub — replaced in Task 10
func newImportCmd() *cobra.Command {
	return &cobra.Command{Use: "import", Short: "Import data from external services (not yet implemented)"}
}
