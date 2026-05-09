package template

import (
	"net/http"
	"text/template"
)

// How to use
// Assume the views are the in folder: ressources/views/
/*
func (c *SampleController) RenderHome() Result {
	return miniView("home.templ",
		Params{"Title": "Min forside"})
}
*/

func View(tmplView string, data map[string]interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		//tmplPath := filepath.Join("resources", "views", "home.templ")
		// Parse templaten
		tmpl, err := template.ParseFiles("resources/views/" + tmplView)
		if err != nil {
			http.Error(w, "Template ikke fundet: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Send result to browseren
		tmpl.Execute(w, data)
	}
}
