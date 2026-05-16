package migration

import (
	"strings"
	"time"

	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
	stubs "github.com/nicklasjeppesen/going_internal/super/cli/stubs"
	"github.com/nicklasjeppesen/going_internal/super/util"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var MigrationCmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "make:migration [migrationName]",
	Short:   "Create a new migration - ex. make:migration posts",
	Long:    `Create a new migration - ex. make:migration posts`,
	Run:     CreateMigration,
}

func CreateMigration(cmd *cobra.Command, args []string) {
	name := args[0]
	timestamp := time.Now().Format("20060102150405")
	util.LoadEnv()
	driver := util.GetEnv("DB_CONNECTION", "")
	stubPath := "migration/migration_" + driver + ".sql.stub"

	stub := stubs.StubDetails{
		Name:        stubPath,
		FileName:    timestamp + "_" + name + ".sql",
		Destination: "./internal/database/migrations/scripts/",
		Values: map[string]string{
			"Model": helper.TextToMultiplum(strings.ToLower(name)),
			"Name":  name,
		},
	}
	stub.CreateStub()
}

func init() {
	// Add the flag to the command
	//ControllerCmd.Flags().BoolVarP(&resource, "resource", "r", false, "Generate a resource controller with CRUD actions")
}
