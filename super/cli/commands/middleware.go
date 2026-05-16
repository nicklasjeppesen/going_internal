package commands

import (
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
	stubs "github.com/nicklasjeppesen/going_internal/super/cli/stubs"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var MiddlewareCmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "make:middleware [middlewareName]",
	Short:   "Create a new middleware - ex. make:middleware Home",
	Long:    `Create a new middleware - ex. make:middleware Home`,
	Run:     CreateMiddlewareCmd,
}

func CreateMiddlewareCmd(cmd *cobra.Command, args []string) {
	name := args[0]

	stubPath := "middleware/middleware.go.stub"

	stub := stubs.StubDetails{
		Name:        stubPath,
		FileName:    name + ".go",
		Destination: "./internal/app/http/middleware/",
		Values: map[string]string{
			"Model": helper.FirstUpper(name),
		},
	}
	stub.CreateStub()
}
