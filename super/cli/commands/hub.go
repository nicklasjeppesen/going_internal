package commands

import (
	"strings"

	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
	stubs "github.com/nicklasjeppesen/going_internal/super/cli/stubs"
	"github.com/spf13/cobra"
)

// HubCmd represents the hub command
var HubCmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "make:hub [hubName]",
	Short:   "Create a new Hub struct - ex. make:hub home",
	Long:    `Create a new Hub struct - ex. make:hub home`,
	Run:     CreateHub,
}

func CreateHub(cmd *cobra.Command, args []string) {
	name := args[0]

	stubPath := "hub/hub.go.stub"

	stub := stubs.StubDetails{
		Name:        stubPath,
		FileName:    name + "_hub.go",
		Destination: "./internal/app/http/hubs/",
		Values: map[string]string{
			"Model": helper.FirstUpper(name) + "Hub",
			"Name":  strings.ToLower(name),
		},
	}
	stub.CreateStub()
}
