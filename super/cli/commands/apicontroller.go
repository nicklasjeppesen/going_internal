package commands

import (
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
	stubs "github.com/nicklasjeppesen/going_internal/super/cli/stubs"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var ControllerAPICmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "make:apicontroller [controllerName]",
	Short:   "Create a new controller struct - ex. make:apicontroller Home",
	Long:    `Create a new controller struct - ex. make:apicontroller Home`,
	Run:     createAPIController,
}

func createAPIController(cmd *cobra.Command, args []string) {
	name := args[0]

	stubPath := "controller/apicontroller.go.stub"

	stub := stubs.StubDetails{
		Name:        stubPath,
		FileName:    name + "_controller.go",
		Destination: "./internal/app/http/controller/",
		Values: map[string]string{
			"ControllerName": helper.FirstUpper(name),
			"Resource":       helper.FirstUpper(helper.TextToMultiplum(name)),
			"Route":          helper.TextToMultiplum(name),
		},
	}
	stub.CreateStub()
}

func init() {
	// Add the flag to the command

}
