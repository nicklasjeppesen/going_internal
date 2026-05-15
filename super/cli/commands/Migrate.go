package commands

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"

	_ "github.com/mattn/go-sqlite3"
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	dbcreator "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	util "github.com/nicklasjeppesen/going_internal/super/util"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var MigrateCmd = &cobra.Command{
	GroupID: groups.MigrateGroup.ID,
	Use:     "migrate",
	Short:   "run migration  - ex. migrate",
	Long:    `run migration  - ex. migrate`,

	Run: func(cmd *cobra.Command, args []string) {

		println("Run migration")
		util.LoadEnv()
		driver := util.GetEnv("DB_CONNECTION", "")
		run_migrations(driver)

	},
}

func init() {
	// Add the flag to the command
	//ControllerCmd.Flags().BoolVarP(&resource, "resource", "r", false, "Generate a resource controller with CRUD actions")
}

const dbpath = "./internal/data/connect.db"
const scriptpath = "./internal/database/migrations/scripts/"

func run_migrations(_driver string) {

	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error trying opening the database: %v", err)
	}
	defer db.Close()
	driver := dbcreator.GetDBConnection(_driver)

	// Sikr at migrations tabellen findes
	_, err = db.Exec(driver.Driver.CreateMigrationTable())
	if err != nil {
		log.Fatalf("Error connection to migration tabel: %v", err)
	}

	// Reading all files in ./scripts folder
	files, err := os.ReadDir(scriptpath)
	if err != nil {
		log.Fatalf("Could not read the folder: %v", err)
	}

	// Filtering only *.SQL
	var migrations []string
	for _, f := range files {
		if !f.IsDir() && filepathExt(f.Name()) == ".sql" {
			migrations = append(migrations, f.Name())
		}
	}

	// Sorting in ASC order
	sort.Strings(migrations)

	// Running each migration
	for _, m := range migrations {

		// Tjek om filen allerede findes i migrations tabellen
		var migrationAlreadyRun bool
		err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM migrations WHERE filename = $1)", m).Scan(&migrationAlreadyRun)
		if err != nil {
			log.Fatalf("Error by checking migration tabel: %v", err)
		}

		if migrationAlreadyRun {
			continue
		}

		fmt.Printf("Running migration: %s\n", m)

		sqlBytes, err := os.ReadFile(scriptpath + m)
		if err != nil {
			log.Fatalf("Could not read %s: %v", m, err)
		}

		_, err = db.Exec(string(sqlBytes))
		if err != nil {
			//log.Fatalf("Error by running %s: %v", m, err)
			fmt.Printf("Error by running %s: %v", m, err)
		}

		// Indsæt i migrations tabellen
		_, err = db.Exec("INSERT INTO migrations (filename) VALUES ($1)", m)
		if err != nil {
			log.Fatalf("Error trying insert migration into migration tabel: %v", err)
		}

	}

	fmt.Println("All Migration executed successfully")
}

func filepathExt(path string) string {
	if len(path) < 4 {
		return ""
	}
	return path[len(path)-4:]
}
