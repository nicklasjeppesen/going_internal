package commands

import (
	"strings"

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

	Run: func(cmd *cobra.Command, args []string) {
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

	},
}

func init() {
	// Add the flag to the command
	//ControllerCmd.Flags().BoolVarP(&resource, "resource", "r", false, "Generate a resource controller with CRUD actions")
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
