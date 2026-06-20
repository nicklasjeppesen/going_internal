package commands

import (
	"context"

	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	"github.com/nicklasjeppesen/going_internal/super/constants"
	dbcreator "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var MigrateCmd = &cobra.Command{
	GroupID: groups.MigrateGroup.ID,
	Use:     "migrate",
	Short:   "run migration  - ex. migrate",
	Long:    `run migration  - ex. migrate`,

	Run: func(cmd *cobra.Command, args []string) {
		println("Running migrations")
		dbCreator := dbcreator.DefaultDBConnection(context.Background())
		dbCreator.Driver.Migrate(constants.DB_MIGRATION_DEFAULT_FILE_PATH)
	},
}

func init() {
	// Add the flag to the command
	//ControllerCmd.Flags().BoolVarP(&resource, "resource", "r", false, "Generate a resource controller with CRUD actions")
}
