package unittest

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	DB "github.com/nicklasjeppesen/going_internal/super/db"
	. "github.com/nicklasjeppesen/going_internal/super/response"
)

// TestSimplCastingOfString test casting of a simple string into json format
// for a valid return value.
func TestSimplCastingOfString(t *testing.T) {

	want := "\"Hej verden\""
	if parsingStruct, err := ToJSON("Hej verden"); err != nil {
		t.Errorf(`StructToJSONWithoutHidden("Hej verden") = %q, %v,`, parsingStruct, err)
	} else if parsingStruct != want {
		t.Errorf(`StructToJSONWithoutHidden("Hej verden") = %q, %v, want match for %#q, nil`, parsingStruct, nil, want)
	}
}

// TestSimplCastingOfSimpleStruct: Calls function to cast stuct to json format
func TestSimplCastingOfSimpleStruct(t *testing.T) {
	user := struct {
		ID   int
		Name string
	}{
		ID:   1,
		Name: "Mads",
	}

	want := "{\"ID\":1,\"Name\":\"Mads\"}"
	if parsingStruct, err := ToJSON(user); err != nil {
		t.Errorf(`StructToJSONWithoutHidden(<simplestruct>) = %q, %v,`, parsingStruct, err)
	} else if parsingStruct != want {
		t.Errorf(`StructToJSONWithoutHidden(<simplestruct>) = %q, %v, want match for %#q, nil`, parsingStruct, nil, want)
	}
}

// TestSimplCastingOfSimpleStruct: Calls function to cast stuct to json format with Tags
func TestSimplCastingOfRealStruct(t *testing.T) {

	id := 1
	name := "Mads"
	var age int64 = 33
	email := "nicklas-jeppesen@live.dk"

	user := struct {
		ID           int
		Name         string `json:"name" validate:"required"`
		Age          int64  `json:"-" validate:"min=0,max=99"`
		Email        string `json:"email" validate:"required"`
		Password     string `json:"password" validate:"required" hidden:"true"`
		SessionToken string `hidden:"true"`
		CSRFToken    string `hidden:"true"`
		Company_id   int64  `json:"Company_id" validate:"required"`
	}{
		ID:           id,
		Name:         name,
		Age:          age,
		Email:        email,
		SessionToken: "Hidden token, shall not been shown",
	}

	want := "{\"-\":" + fmt.Sprint(age) + ",\"Company_id\":0,\"ID\":" + fmt.Sprint(id) + ",\"email\":\"" + fmt.Sprint(email) + "\",\"name\":\"" + fmt.Sprint(name) + "\"}"
	if parsingStruct, err := ToJSON(user); err != nil {
		t.Errorf(`StructToJSONWithoutHidden(<simplestruct>) = %q, %v,`, parsingStruct, err)
	} else if parsingStruct != want {
		t.Errorf(`TestSimplCastingOfRealStruct = %q, %v, want match for %#q, nil`, parsingStruct, nil, want)
	}
}

// Data
type User struct {
	DB.ActiveRecord[*User]
	Name         string `json:"name" validate:"required"`
	Age          int64  `json:"age" validate:"min=0,max=99"`
	Email        string `json:"email" validate:"required"`
	Password     string `json:"password" validate:"required" hidden:"true"`
	SessionToken string `hidden:"true"`
	CSRFToken    string `hidden:"true"`
	Company_id   int64  `json:"Company_id" validate:"required"`
}

func (_user User) DB() *User {
	user := &User{}
	user.Table = "users"
	user.Columns = map[string]any{
		"name": &user.Name,
		"age":  &user.Age,
	}

	// Creating DB
	user.ParentDB = DB.CreateORM(user)
	return user
}

func TestSimplCastingOfRealStructWithActiveRecord(t *testing.T) {

	var randomAge = rand.Intn(100)

	user := User{
		Name:         "Mads",
		Age:          int64(randomAge),
		Email:        "nicklas-jeppesen@live.dk",
		SessionToken: "Hidden token, shall not been shown",
	}.DB()

	var randomInt = rand.Intn(100)
	var now = time.Now()
	var later = now.Add(1)
	user.SystemMapper().ValueHolder["id"].Setter(int64(randomInt))
	user.Created_at = now
	user.Updated_at = later
	user.DBSetUp().ValueHolder["age"].Setter(int64(randomAge))
	user.DBSetUp().ValueHolder["name"].Setter("Mads")

	want := "{\"Company_id\":0,\"SystemFields\":{\"Columns\":{},\"Created_at\":\"" + fmt.Sprint(now) + "\",\"Id\":" + fmt.Sprint(randomInt) + ",\"Name\":\"\",\"Pivots\":{},\"Routes\":{},\"Table\":\"users\",\"Updated_at\":\"" + fmt.Sprint(later) + "\"},\"age\":" + fmt.Sprint(randomAge) + ",\"email\":\"\",\"name\":\"Mads\"}"

	if parsingStruct, err := ToJSON(user); err != nil {
		t.Errorf(`StructToJSONWithoutHidden(<simplestruct>) = %q, %v,`, parsingStruct, err)
	} else if parsingStruct != want {
		t.Errorf(`StructToJSONWithoutHidden(<simplestruct>) = %q, %v, want match for /n\n %q, nil`, parsingStruct, nil, want)
	}
}

// Run all test in this package and below
// go test ./tests/... // kører Alle test i mappen test.

// Print kun pakker med tests: go test $(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./internal/super/...)
// Eller lav Alias: alias gts='go test $(go list -f "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}" ./internal/super/...)'
