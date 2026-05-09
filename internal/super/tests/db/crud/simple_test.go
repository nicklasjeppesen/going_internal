package cases

import (
	"fmt"
	"log"
	. "myapp/internal/super/tests/db/shared"
	data "myapp/internal/super/tests/db/shared/data"
	model "myapp/internal/super/tests/db/shared/models"
	schema "myapp/internal/super/tests/db/shared/schema"

	"testing"
)

func TestSimpleInMemoryDB(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}
	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
		t.Errorf(`TestSimpleInMemoryDB fail of insert simple JsonData with errors %v`, err.Error())
	}

	row := database.DB.QueryRow("select * from users limit 1")
	values := make([]any, 6)

	for i := range values {
		values[i] = new(any) // create addressable placeholder
	}

	if errr := row.Scan(values[:]...); errr != nil {
		t.Errorf(`TestSimpleInMemoryDB fail of insert simple JsonData with errors %v`, errr.Error())
	}
	id := *values[0].(*any)
	name := *values[1].(*any)
	age := *values[2].(*any)

	if id.(int64) == 1 && name.(string) == "Nicklas" && age.(int64) == 30 {

	} else {
		t.Errorf(`TestSimpleInMemoryDB fail of insert simple JsonData with`)
	}

	database.DB.Close()
	fmt.Println("User inserted successfully")
}

func TestFirstOfModelData(t *testing.T) {
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

	user := model.User{}.DB()
	user.ParentDB.SetDbConn(database.DB)
	var result = user.First()

	if result.IsEmpty() || user.Name != "Nicklas" || user.Age != int64(30) {
		t.Errorf(`TestFirstOfModelData fail of calling First, result is empty`)
	}

	database.DB.Close()
	fmt.Println("User inserted successfully")
}

func TestCreateAndFirstOfModelData(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
		t.Errorf(`StructToJSONWithoutHidden(<simplestruct>) nil`)
	}

	user := model.User{}.DB()
	user.Name = "nicklas2"
	user.Age = 25
	user.DB()
	user.ParentDB.SetDbConn(database.DB)
	_, err = user.Save()

	if err != nil {
		t.Errorf(`TestCreateAndFirstOfModelData with saving data cause error %v`, err.Error())
	}

	secondResult := user.Where("name", "nicklas2").First()
	fmt.Println(secondResult.Name)

	if secondResult.IsEmpty() || secondResult.Name != "nicklas2" {
		t.Errorf(`TestCreateAndFirstOfModelData error by saving objects`)
	}

	defer database.DB.Close()
	fmt.Println("User inserted successfully")
}
