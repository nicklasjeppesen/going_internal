package cases

import (
	"fmt"
	"log"
	"testing"

	. "github.com/nicklasjeppesen/going_internal/internal/super/tests/db/shared"
	"github.com/nicklasjeppesen/going_internal/internal/super/tests/db/shared/data"
	model "github.com/nicklasjeppesen/going_internal/internal/super/tests/db/shared/models"
	schema "github.com/nicklasjeppesen/going_internal/internal/super/tests/db/shared/schema"
)

func TestFirstOfHasOne(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
		t.Errorf(`TestFirstOfModelData fail of load JsonData with errors %v`, err.Error())
	}

	if err := database.LoadSchema(schema.CompanySchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.CompanyJSON, "companies"); err != nil {
		log.Fatal(err)
		t.Errorf(`TestFirstOfModelData fail of load JsonData with errors %v`, err.Error())
	}

	user := model.User{}.DB()
	var result = user.With("company").First()

	if result.IsEmpty() || user.Name != "Nicklas" || user.Age != int64(30) {
		t.Errorf(`TestFirstOfModelData fail of calling First, result is empty`)
	}

	if result.Company.IsEmpty() || result.Company.Name != "Company One" {
		t.Errorf(`TestFirstOfModelData fail of calling First, result is empty`)
	}

	database.DB.Close()
	fmt.Println("User inserted successfully")
}
