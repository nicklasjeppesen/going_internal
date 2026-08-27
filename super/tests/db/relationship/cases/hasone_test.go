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

func TestFirstOfCompany(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.CompanySchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.CompanyJSON, "companies"); err != nil {
		t.Errorf(`TestFirstOfCompany fail of load JsonData with errors %v`, err.Error())
	}

	ctx := context.Background()
	company := model.Company{}.DB(ctx)
	var result = company.First()

	if result.IsEmpty() || result.Name != "Company One" {
		t.Errorf(`TestFirstOfCompany: expected Name="Company One", got Name=%s IsEmpty=%v`, result.Name, result.IsEmpty())
	}

	database.DB.Close()
}

func TestSaveNewCompany(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.CompanySchema); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	company := model.Company{}.DB(ctx)
	company.Name = "New Company"

	saved, err := company.Save()
	if err != nil {
		t.Errorf(`TestSaveNewCompany: error saving: %v`, err.Error())
	}

	if saved.Name != "New Company" {
		t.Errorf(`TestSaveNewCompany: expected Name="New Company", got Name=%s`, saved.Name)
	}

	if saved.Id == 0 {
		t.Errorf(`TestSaveNewCompany: expected non-zero Id after save`)
	}

	database.DB.Close()
}

func TestCompanyGetAll(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.CompanySchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.CompanyJSON, "companies"); err != nil {
		t.Errorf(`TestCompanyGetAll fail of load JsonData with errors %v`, err.Error())
	}

	ctx := context.Background()
	company := model.Company{}.DB(ctx)
	results := company.Get()

	if len(results) == 0 {
		t.Errorf(`TestCompanyGetAll: expected at least 1 company, got 0`)
	}

	if results[0].Name != "Company One" {
		t.Errorf(`TestCompanyGetAll: expected first company Name="Company One", got Name=%s`, results[0].Name)
	}

	database.DB.Close()
}

func TestUserBelongsToCompany(t *testing.T) {
	database, err := NewInMemoryDB()
	if err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.UserSchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadSchema(schema.CompanySchema); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		log.Fatal(err)
	}

	if err := database.LoadJsonData(data.CompanyJSON, "companies"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	company := model.Company{}.DB(ctx)
	companyResult := company.First()

	user := model.User{}.DB(ctx)
	userResult := user.First()

	if userResult.Company_id != float64(companyResult.Id) {
		t.Errorf(`TestUserBelongsToCompany: user company_id=%v should match company id=%v`, userResult.Company_id, companyResult.Id)
	}

	database.DB.Close()
}
