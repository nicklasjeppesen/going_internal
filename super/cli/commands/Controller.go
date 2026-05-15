package commands

import (
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
	"github.com/spf13/cobra"
)

var resource bool // Variabel to represent ressource flag

// ControllerCmd represents the Controller command
var ControllerCmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "make:controller [controllerName]",
	Short:   "Create a new controller class - ex. make:controller HomeController",
	Long:    `Create a new controller class - ex. make:controller HomeController`,

	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		stubPath := "controller/controller.go.stub"
		if resource {
			stubPath = "controller/controllerRessource.go.stub"
		}

		stub := helper.StubDetails{
			Name:        stubPath,
			FileName:    name + "Controller.go",
			Destination: "./internal/app/http/controller/",
			Values: map[string]string{
				"Model": name + "Controller",
				"Name":  name,
			},
		}
		stub.CreateStub()

	},
}

func init() {
	// Add the flag to the command
	ControllerCmd.Flags().BoolVarP(&resource, "resource", "r", false, "Generate a resource controller with CRUD actions")
}
