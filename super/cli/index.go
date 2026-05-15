package cli

import (
	commands "github.com/nicklasjeppesen/going_internal/super/cli/commands"
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"

	"github.com/spf13/cobra"
)

func GetGroups() []*cobra.Group {

	return []*cobra.Group{
		groups.GeneratorGroup,
	}
}

func GetCommands() []*cobra.Command {
	return []*cobra.Command{
		commands.ControllerCmd,
		commands.ModelCmd,
	}
}
