package commands

import (
	"strings"

	migration "github.com/nicklasjeppesen/going_internal/super/cli/commands/migration"
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
	stubs "github.com/nicklasjeppesen/going_internal/super/cli/stubs"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var ModelCmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "make:model [modelName]",
	Short:   "Create a new model class - ex. make:model Home",
	Long:    `Create a new model class - ex. make:model Home`,
	Run:     CreateModel,
}

var (
	createMigration  bool
	createController bool
)

// TODO table name is properly wrong, if it created automated, missing an s at the end
func CreateModel(cmd *cobra.Command, args []string) {
	name := args[0]
	modelName := helper.FirstUpper(name)
	variableName := helper.AllLower(name)

	stubPath := "model/model.go.stub"

	stub := stubs.StubDetails{
		Name:        stubPath,
		FileName:    name + ".go",
		Destination: "./internal/app/models/db/",
		Values: map[string]string{
			"Model":        modelName,
			"variableName": variableName,
			"tableName":    helper.TextToMultiplum(strings.ToLower(name)),
		},
	}
	stub.CreateStub()

	if createMigration {
		migration.CreateMigration(cmd, args)
	}
	if createController {
		CreateController(cmd, args)
	}

}

func init() {
	// Add the flag to the command
	ModelCmd.Flags().BoolVarP(&createMigration, "migration", "m", false, "Generate a migration file")
	ModelCmd.Flags().BoolVarP(&createController, "Controller", "c", false, "Generate a controller")
	//ModelCmd.Flags().BoolVarP(&resource, "resource", "r", false, "Generate a migration file")

}
