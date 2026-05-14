package groups

import "github.com/spf13/cobra"

var (
	GeneratorGroup = &cobra.Group{
		ID:    "generation",
		Title: "Generation Commands (Artisan style):",
	}
)
