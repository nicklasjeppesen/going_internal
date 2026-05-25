package template

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"text/template"

	"github.com/gorilla/sessions"
	"github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/util"
)

// How to use
// Assume the views are the in folder: ressources/views/
/*
func (c *SampleController) RenderHome() Result {
	return View("home",
		Params{"Title": "Min forside"})
}
*/
type viewparam = map[string]any

type TemplateView struct {
	CustomViewFunctions template.FuncMap
}

func (viewtemplate TemplateView) View(tmplView string, prop ...viewparam) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		data := getData(r, w, prop...)

		// Define 'old' funktionen.
		funcMap := template.FuncMap{
			"old": func(key string) any {
				if val, exists := data[constants.Old]; exists {
					if oldMap, ok := val.(map[string]string); ok {
						if oldVal, found := oldMap[key]; found {
							return oldVal
						}
					}
				}
				return "" // Return empty string if value does not exists
			},
		}
		maps.Copy(viewtemplate.CustomViewFunctions, funcMap)
		getTemplate(tmplView, viewtemplate.CustomViewFunctions).ExecuteTemplate(w, "base", data)

	}
}

func getData(r *http.Request, w http.ResponseWriter, prop ...viewparam) map[string]any {
	var data = make(map[string]any)

	if len(prop) > 0 && prop[0] != nil {
		data = prop[0]
	}

	data = addErrors(data, w, r)
	data = addOld(data, w, r)

	// add old post values to the view data, if there is any in the session
	data[constants.Csrf_token] = r.Context().Value(constants.Csrf_token)
	return data
}

func getTemplate(tmplView string, funcMap template.FuncMap) *template.Template {
	// Parse templaten

	tmpl, err := template.New("internal/resources/views/base.template").Funcs(funcMap).ParseFiles(
		"internal/resources/views/base.template",
		"internal/resources/views/"+tmplView+".template")
	if err != nil {
		panic("Template view was not found: " + err.Error())
	}

	return tmpl
}

func addOld(propVal map[string]any, w http.ResponseWriter, r *http.Request) map[string]any {
	return addViewData(propVal, w, r, constants.Old)
}

// add errors infomation to the view data, if there is any in the session
func addErrors(propVal map[string]any, w http.ResponseWriter, r *http.Request) map[string]any {
	return addViewData(propVal, w, r, constants.Errors)
}

// add errors infomation to the view data, if there is any in the session
func addViewData(propVal map[string]any, w http.ResponseWriter, r *http.Request, name string) map[string]any {
	var key = util.GetEnv(constants.APP_Key, "")
	var store = sessions.NewCookieStore([]byte(key))

	session, err := store.Get(r, constants.Session_info)
	if err != nil {
		fmt.Println("Fejl ved hentning af session:", err)
	}

	if messages, ok := session.Values[name].(string); ok {
		var b2 = []byte(messages)
		var m2 map[string]string

		err = json.Unmarshal(b2, &m2)
		if err != nil {
			fmt.Println(err.Error())
		}

		propVal[name] = m2
		propVal["has"+name] = true

		// Deelte the message, so it will not be shown again, after next relad (flash-message)
		delete(session.Values, name)
		session.Options.Path = "/" // Sikrer samme sti
		session.Save(r, w)
	}
	return propVal
}
