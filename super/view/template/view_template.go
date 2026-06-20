package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

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

var templates = map[string]*template.Template{}

type viewparam = map[string]any

type TemplateView struct {
	CustomViewFunctions template.FuncMap
	BaseView            string
}

func (viewtemplate TemplateView) View(tmplView string, prop ...viewparam) func(http.ResponseWriter, *http.Request) {
	if templates[viewtemplate.BaseView] == nil {
		templates[viewtemplate.BaseView] = New().templates
	}

	baseView := viewtemplate.BaseView
	if viewtemplate.BaseView == "" {
		baseView = tmplView
	}

	return func(w http.ResponseWriter, r *http.Request) {
		data := getData(r, w, tmplView, prop...)

		// 1. Check if the template/block even exists in the general map
		tmpl := templates[viewtemplate.BaseView]
		if tmpl == nil || tmpl.Lookup(baseView) == nil {
			http.Error(w, "Template error: Could not find view '"+baseView+"'. Check for spelling eror in {{ define }}?", http.StatusInternalServerError)
			return
		}

		// 2. Check if the specific tmpView exists,
		if tmpl.Lookup(tmplView) == nil {
			http.Error(w, "Template error: Could not find view '"+tmplView+"'. Check for spelling eror in {{ define \""+tmplView+"\" }}", http.StatusInternalServerError)
			return
		}

		var buf bytes.Buffer

		if err := templates[viewtemplate.BaseView].ExecuteTemplate(&buf, baseView, data); err != nil {
			// TODO: Place with proper error handling
			fmt.Println("error", err.Error())
			http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		buf.WriteTo(w)
	}
}

func getData(r *http.Request, w http.ResponseWriter, tmplView string, prop ...viewparam) map[string]any {
	var data = make(map[string]any)

	if len(prop) > 0 && prop[0] != nil {
		data = prop[0]
	}

	data = addErrors(data, w, r)
	data = addOld(data, w, r)
	data = addFlash(data, w, r)
	data[constants.Csrf_token] = r.Context().Value(constants.Csrf_token)
	data["ContentView"] = tmplView
	return data
}

func addFlash(propVal map[string]any, w http.ResponseWriter, r *http.Request) map[string]any {
	return addViewData(propVal, w, r, constants.Flash)
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
		var m2 map[string]any

		err = json.Unmarshal(b2, &m2)
		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println("has" + name)
		propVal[name] = m2
		propVal["has"+name] = true

		// Delete the message, so it will not be shown again, after next reload (flash-message)
		delete(session.Values, name)
		session.Options.Path = "/" // Sikrer samme sti
		session.Save(r, w)
	} else {
		propVal[name] = map[string]any{}
		propVal["has"+name] = false

	}
	return propVal
}
