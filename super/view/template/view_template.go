package template

import (
	"net/http"
	"text/template"
)

// How to use
// Assume the views are the in folder: ressources/views/
/*
func (c *SampleController) RenderHome() Result {
	return View("home",
		Params{"Title": "Min forside"})
}
*/

func View(tmplView string, data map[string]interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Parse templaten
		tmpl, err := template.ParseFiles("internal/resources/views/" + tmplView + ".template")
		if err != nil {
			http.Error(w, "Template view was not found: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Send result to browseren
		tmpl.Execute(w, data)
	}
}
