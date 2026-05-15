package drivers

import (
	"github.com/nicklasjeppesen/going_internal/super/constants"
	types "github.com/nicklasjeppesen/going_internal/super/db/types"
	"github.com/nicklasjeppesen/going_internal/super/util"
)

/*
*
* Return the default DB connnection based on the env file
* Maybe this should be removed to factory in Driver
 */
func DefaultDBConnection() types.DBCreator {
	var DBConnection = util.GetEnv(constants.DB_CONNECTION, "")
	return GetDBConnection(DBConnection)
}

/*
*
* Return the default DB connnection based on the env file
* Maybe this should be removed to factory in Driver
 */
func GetDBConnection(driver string) types.DBCreator {
	switch driver {
	case "sqlite":
		return CreateSQLite()
	case "Postgres":
		return CreatePostgressDB()
	default:
		panic("No database driver exists")
	}
}
