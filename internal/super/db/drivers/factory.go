package drivers

import (
	"myapp/internal/super/constants"
	types "myapp/internal/super/db/types"
	"myapp/internal/super/util"
)

/*
*
* Return the default DB connnection based on the env file
* Maybe this should be removed to factory in Driver
 */
func DefaultDBConnection() types.DBCreator {
	var DBConnection = util.GetEnv(constants.DB_CONNECTION, "")
	switch DBConnection {
	case "SQLite":
		return CreateSQLite()
	case "Postgres":
		return CreatePostgressDB()
	default:
		return CreateSQLite()
	}
}
