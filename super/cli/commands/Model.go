package commands

import (
	"strings"

	migration "github.com/nicklasjeppesen/going_internal/super/cli/commands/migration"
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
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
	modelName := FirstUpper(name)
	variableName := AllLower(name)

	stubPath := "model/model.go.stub"

	stub := helper.StubDetails{
		Name:        stubPath,
		FileName:    name + ".go",
		Destination: "./internal/app/models/db/",
		Values: map[string]string{
			"Model":        modelName,
			"variableName": variableName,
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
	//ModelCmd.Flags().BoolVarP(&resource, "resource", "ressource", false, "Generate a migration file")
}

func FirstUpper(s string) string {
	s = strings.ToLower(s)
	if s == "" {
		return s
	}

	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}

func AllLower(s string) string {
	return strings.ToLower(s)
}
