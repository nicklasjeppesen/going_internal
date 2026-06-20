package drivers

import (
	"context"

	"github.com/nicklasjeppesen/going_internal/super/constants"
	types "github.com/nicklasjeppesen/going_internal/super/db/types"
	"github.com/nicklasjeppesen/going_internal/super/util"
)

// Return the default DB connnection based on the env file
// Maybe this should be removed to factory in Driver
func DefaultDBConnection(ctx context.Context) types.DBCreator {
	var DBConnection = util.GetEnv(constants.DB_CONNECTION, "")
	return GetDBConnection(DBConnection, ctx)
}

// Return the default DB connnection based on the env file
// Maybe this should be removed to factory in Driver
func GetDBConnection(driver string, ctx context.Context) types.DBCreator {
	switch driver {
	case "sqlite":
		return CreateSQLite(ctx)
	case "Postgres":
		return CreatePostgressDB(ctx)
	default:
		panic("No database driver exists")
	}
}
