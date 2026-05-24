package commands

import (
	"github.com/FluidifyAI/Regen/backend/internal/enterprise"
	"github.com/spf13/cobra"
)

// proHooks holds the enterprise extension points used by the serve command.
// Defaults to no-op stubs; regen-pro overrides via SetEnterpriseHooks.
var proHooks = enterprise.NewNoOp()

// SetEnterpriseHooks replaces the default no-op hooks with Pro implementations.
// Must be called before Execute().
func SetEnterpriseHooks(h enterprise.Hooks) {
	proHooks = h
}

// NewRootCmd returns the cobra root command. All subcommands are attached here.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "regen",
		Short: "Fluidify Regen — open-source incident management platform",
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newMigrateCmd())

	return root
}
