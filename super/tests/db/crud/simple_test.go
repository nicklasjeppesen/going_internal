package cases

import (
	"context"
	"log"
	"testing"

	. "github.com/nicklasjeppesen/going_internal/super/tests/db/shared"
	"github.com/nicklasjeppesen/going_internal/super/tests/db/shared/data"
	model "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/models"
	schema "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/schema"
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
		t.Errorf(`TestSimpleInMemoryDB fail of insert simple JsonData with errors %v`, err.Error())
	}

	row := database.DB.QueryRow("select * from users limit 1")
	values := make([]any, 6)

	for i := range values {
		values[i] = new(any)
	}

	if errr := row.Scan(values[:]...); errr != nil {
		t.Errorf(`TestSimpleInMemoryDB fail of insert simple JsonData with errors %v`, errr.Error())
	}
	id := *values[0].(*any)
	name := *values[1].(*any)
	age := *values[2].(*any)

	if id.(int64) != 1 || name.(string) != "Nicklas" || age.(int64) != 30 {
		t.Errorf(`TestSimpleInMemoryDB fail: got id=%v name=%v age=%v, want id=1 name=Nicklas age=30`, id, name, age)
	}

	database.DB.Close()
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
	}

	ctx := context.Background()
	user := model.User{}.DB(ctx)
	var result = user.First()

	if result.IsEmpty() || result.Name != "Nicklas" || result.Age != int64(30) {
		t.Errorf(`TestFirstOfModelData fail: expected Name=Nicklas Age=30, got Name=%s Age=%d IsEmpty=%v`, result.Name, result.Age, result.IsEmpty())
	}

	database.DB.Close()
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
	}

	ctx := context.Background()

	user := model.User{}.DB(ctx)
	user.Name = "nicklas2"
	user.Age = 25
	_, err = user.Save()
	if err != nil {
		t.Errorf(`TestCreateAndFirstOfModelData error saving: %v`, err.Error())
	}

	user2 := model.User{}.DB(ctx)
	secondResult := user2.Where("name", "nicklas2").First()

	if secondResult.IsEmpty() || secondResult.Name != "nicklas2" {
		t.Errorf(`TestCreateAndFirstOfModelData error: expected Name=nicklas2, got Name=%s IsEmpty=%v`, secondResult.Name, secondResult.IsEmpty())
	}

	database.DB.Close()
}

func TestGetAllUsers(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	user := model.User{}.DB(ctx)
	results := user.Get()

	if len(results) == 0 {
		t.Errorf(`TestGetAllUsers: expected at least 1 user, got 0`)
	}

	if results[0].Name != "Nicklas" {
		t.Errorf(`TestGetAllUsers: expected first user Name=Nicklas, got Name=%s`, results[0].Name)
	}

	database.DB.Close()
}

func TestWhereWithOperator(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	user := model.User{}.DB(ctx)
	result := user.Where("age", ">", 25).First()

	if result.IsEmpty() || result.Name != "Nicklas" {
		t.Errorf(`TestWhereWithOperator: expected Name=Nicklas, got Name=%s IsEmpty=%v`, result.Name, result.IsEmpty())
	}

	database.DB.Close()
}

func TestFirstReturnsEmptyForNoMatch(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	user := model.User{}.DB(ctx)
	result := user.Where("name", "nonexistent").First()

	if !result.IsEmpty() {
		t.Errorf(`TestFirstReturnsEmptyForNoMatch: expected IsEmpty=true, got IsEmpty=%v Name=%s`, result.IsEmpty(), result.Name)
	}

	database.DB.Close()
}

func TestUpdateExistingUser(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	user := model.User{}.DB(ctx)
	user.Where("name", "Nicklas").First()
	user.Name = "UpdatedNicklas"
	err = user.Update()
	if err != nil {
		t.Errorf(`TestUpdateExistingUser: error updating: %v`, err.Error())
	}

	user2 := model.User{}.DB(ctx)
	result := user2.Where("name", "UpdatedNicklas").First()

	if result.IsEmpty() || result.Name != "UpdatedNicklas" {
		t.Errorf(`TestUpdateExistingUser: expected Name=UpdatedNicklas, got Name=%s IsEmpty=%v`, result.Name, result.IsEmpty())
	}

	database.DB.Close()
}

func TestDeleteExistingUser(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	user := model.User{}.DB(ctx)
	user.Where("name", "Nicklas").First()

	err = user.Delete()
	if err != nil {
		t.Errorf(`TestDeleteExistingUser: error deleting: %v`, err.Error())
	}

	user2 := model.User{}.DB(ctx)
	result := user2.Where("name", "Nicklas").First()

	if !result.IsEmpty() {
		t.Errorf(`TestDeleteExistingUser: expected IsEmpty=true after delete, got IsEmpty=%v`, result.IsEmpty())
	}

	database.DB.Close()
}

func TestSaveNewUser(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	user := model.User{}.DB(ctx)
	user.Name = "NewUser"
	user.Age = 20

	saved, err := user.Save()
	if err != nil {
		t.Errorf(`TestSaveNewUser: error saving: %v`, err.Error())
	}

	if saved.Name != "NewUser" {
		t.Errorf(`TestSaveNewUser: expected Name=NewUser, got Name=%s`, saved.Name)
	}

	if saved.Id == 0 {
		t.Errorf(`TestSaveNewUser: expected non-zero Id after save`)
	}

	database.DB.Close()
}
