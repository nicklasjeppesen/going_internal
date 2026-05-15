package groups

import "github.com/spf13/cobra"

var (
	GeneratorGroup = &cobra.Group{
		ID:    "generation",
		Title: "Make Commands",
	}
	MigrateGroup = &cobra.Group{
		ID:    "Migrate",
		Title: "Migrate",
	}
)
