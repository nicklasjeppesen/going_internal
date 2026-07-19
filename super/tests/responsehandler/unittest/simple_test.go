package unittest

import (
	"fmt"
	"testing"

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

// TestSimplCastingOfRealStruct: Calls function to cast stuct to json format with Tags
func TestSimplCastingOfRealStruct(t *testing.T) {

	id := 1
	name := "Mads"
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
		Age:          33,
		Email:        email,
		SessionToken: "Hidden token, shall not been shown",
	}

	want := "{\"ID\":" + fmt.Sprint(id) + ",\"name\":\"" + fmt.Sprint(name) + "\",\"email\":\"" + fmt.Sprint(email) + "\",\"password\":\"\",\"SessionToken\":\"Hidden token, shall not been shown\",\"CSRFToken\":\"\",\"Company_id\":0}"
	if parsingStruct, err := ToJSON(user); err != nil {
		t.Errorf(`StructToJSONWithoutHidden(<simplestruct>) = %q, %v,`, parsingStruct, err)
	} else if parsingStruct != want {
		t.Errorf(`TestSimplCastingOfRealStruct = %q, %v, want match for %#q, nil`, parsingStruct, nil, want)
	}
}

func TestSimplCastingOfRealStructWithActiveRecord(t *testing.T) {
	t.Skip("Requires a database connection - integration test")
}

// Run all test in this package and below
// go test ./tests/... // kører Alle test i mappen test.

// Print kun pakker med tests: go test $(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./internal/super/...)
// Eller lav Alias: alias gts='go test $(go list -f "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}" ./internal/super/...)'
