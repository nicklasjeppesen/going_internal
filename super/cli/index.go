package cli

import (
	commands "github.com/nicklasjeppesen/going_internal/super/cli/commands"
	migrate "github.com/nicklasjeppesen/going_internal/super/cli/commands/migrate"
	migration "github.com/nicklasjeppesen/going_internal/super/cli/commands/migration"
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"

	"github.com/spf13/cobra"
)

func GetGroups() []*cobra.Group {

	return []*cobra.Group{
		groups.GeneratorGroup,
		groups.MigrateGroup,
	}
}

func GetCommands() []*cobra.Command {
	return []*cobra.Command{
		commands.ControllerCmd,
		commands.ModelCmd,
		migrate.MigrateCmd,
		migration.MigrationCmd,
		commands.HubCmd,
		commands.MiddlewareCmd,
	}
}
