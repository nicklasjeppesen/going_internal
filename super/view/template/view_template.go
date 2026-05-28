package template

import (
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

var templates *template.Template

/*
	func Init(CustomViewFunctions template.FuncMap) {
		// Vi starter med at lave en tom template-samling
		if templates != nil {
			return
		}

		var funcMap = template.FuncMap{}
		if CustomViewFunctions != nil {
			funcMap = CustomViewFunctions
		}

		funcMap["old"] = func(key string, data map[string]any) any {
			if val, exists := data[constants.Old]; exists {
				if oldMap, ok := val.(map[string]string); ok {
					if oldVal, found := oldMap[key]; found {
						return oldVal
					}
				}
			}
			return "" // Return empty string if value does not exists
		}

		funcMap["render"] = func(name string, data any) template.HTML {
			var buf bytes.Buffer

			err := templates.ExecuteTemplate(&buf, name, data)
			if err != nil {
				return ""
			}

			return template.HTML(buf.String())
		}

		templates = template.New("")
		templates.Funcs(funcMap)

		// filepath.WalkDir gennemgår ALLE filer og undermapper i "view"
		err := filepath.WalkDir("internal/resources/views", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Vi er kun interesserede i .html filer
			if !d.IsDir() && strings.HasSuffix(path, ".template") {
				// Indlæs filen og tilføj den til vores template-samling
				_, err = templates.ParseFiles(path)
				if err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			panic("Fejl ved indlæsning af templates: " + err.Error())
		}
	}
*/
type viewparam = map[string]any

type TemplateView struct {
	CustomViewFunctions template.FuncMap
	BaseView            string
}

func (viewtemplate TemplateView) View(tmplView string, prop ...viewparam) func(http.ResponseWriter, *http.Request) {
	if templates == nil {
		templates = New().templates
	}

	return func(w http.ResponseWriter, r *http.Request) {
		data := getData(r, w, tmplView, prop...)
		templates.ExecuteTemplate(w, viewtemplate.BaseView, data)

	}
}

func getData(r *http.Request, w http.ResponseWriter, tmplView string, prop ...viewparam) map[string]any {
	var data = make(map[string]any)

	if len(prop) > 0 && prop[0] != nil {
		data = prop[0]
	}

	data = addErrors(data, w, r)
	data = addOld(data, w, r)
	data[constants.Csrf_token] = r.Context().Value(constants.Csrf_token)
	data["ContentView"] = tmplView
	return data
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

		// Delete the message, so it will not be shown again, after next reload (flash-message)
		delete(session.Values, name)
		session.Options.Path = "/" // Sikrer samme sti
		session.Save(r, w)
	}
	return propVal
}

/*
func getTemplate(tmplView string, funcMap template.FuncMap) *template.Template {

		var tmpl *template.Template

		funcMap["render"] = func(name string, data any) template.HTML {
			var buf bytes.Buffer

			err := tmpl.ExecuteTemplate(&buf, name, data)
			if err != nil {
				return ""
			}

			return template.HTML(buf.String())
		}

		tmpl, _ = template.Must(
			template.New("").
				Funcs(funcMap).
				ParseGlob(filepath.Join("internal/resources/views", "*.template")),
		).ParseGlob(filepath.Join("internal/resources/views/auth", "*.template"))

		return tmpl
	}
*/
