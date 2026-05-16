package commands

import (
	"strings"

	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
	stubs "github.com/nicklasjeppesen/going_internal/super/cli/stubs"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var JobsCmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "make:job [jobName]",
	Short:   "Create a new job struct - ex. make:job Home",
	Long:    `Create a new job struct - ex. make:job Home`,
	Run:     CreateJobs,
}

func CreateJobs(cmd *cobra.Command, args []string) {
	name := args[0]

	stubPath := "jobs/job.go.stub"

	stub := stubs.StubDetails{
		Name:        stubPath,
		FileName:    strings.ToLower(name) + ".go",
		Destination: "./internal/app/jobs/",
		Values: map[string]string{
			"Model": helper.FirstUpper(name),
			"Name":  name,
		},
	}
	stub.CreateStub()
}
