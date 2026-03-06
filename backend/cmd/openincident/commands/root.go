package commands

import "github.com/spf13/cobra"

// NewRootCmd returns the cobra root command. All subcommands are attached here.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "openincident",
		Short: "Fluidify Alert — open-source incident management platform",
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newMigrateCmd())

	return root
}
