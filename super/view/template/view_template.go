package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"

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

// engines holds the parsed templates per base view. They are parsed lazily
// on first use, which can happen from several requests at once, so the map is
// guarded by enginesMu.
var (
	enginesMu sync.RWMutex
	engines   = map[string]*Engine{}
)

// engineFor returns the template engine for the view's base view, parsing
// the templates the first time.
func engineFor(viewtemplate TemplateView) *Engine {
	enginesMu.RLock()
	engine := engines[viewtemplate.BaseView]
	enginesMu.RUnlock()
	if engine != nil {
		return engine
	}

	enginesMu.Lock()
	defer enginesMu.Unlock()
	if engine = engines[viewtemplate.BaseView]; engine == nil {
		engine = New(viewtemplate.CustomViewFunctions)
		engines[viewtemplate.BaseView] = engine
	}
	return engine
}

// templatesFor returns the parsed templates for the view's base view.
func templatesFor(viewtemplate TemplateView) *template.Template {
	return engineFor(viewtemplate).templates
}

// templateError writes a template error page. In debug mode it also lists
// the templates that failed to parse, which is usually why a view is missing.
func templateError(w http.ResponseWriter, engine *Engine, message string) {
	if util.GetEnv(constants.APP_Debug, "") == "true" {
		message += engine.parseErrorDetails()
	}
	http.Error(w, message, http.StatusInternalServerError)
}

type viewparam = map[string]any

type TemplateView struct {
	CustomViewFunctions template.FuncMap
	BaseView            string
}

func (viewtemplate TemplateView) View(tmplView string, prop ...viewparam) func(http.ResponseWriter, *http.Request) {
	engine := engineFor(viewtemplate)
	tmpl := engine.templates

	baseView := viewtemplate.BaseView
	if viewtemplate.BaseView == "" {
		baseView = tmplView
	}

	return func(w http.ResponseWriter, r *http.Request) {
		data := getData(r, w, tmplView, prop...)

		// 1. Check if the template/block even exists in the general map
		if tmpl == nil || tmpl.Lookup(baseView) == nil {
			templateError(w, engine, "Template error: Could not find view '"+baseView+"'. Check for spelling eror in {{ define }}?")
			return
		}

		// 2. Check if the specific tmpView exists,
		if tmpl.Lookup(tmplView) == nil {
			templateError(w, engine, "Template error: Could not find view '"+tmplView+"'. Check for spelling eror in {{ define \""+tmplView+"\" }}")
			return
		}

		var buf bytes.Buffer

		if err := tmpl.ExecuteTemplate(&buf, baseView, data); err != nil {
			// TODO: Place with proper error handling
			fmt.Println("error", err.Error())
			templateError(w, engine, "Template error: "+err.Error())
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

	// Errors, old input and flash messages live in the same session cookie:
	// read it once, and save it once if any of them were consumed.
	session, err := getSessionStore().Get(r, constants.Session_info)
	if err != nil {
		fmt.Println("Fejl ved hentning af session:", err)
	}

	consumed := addErrors(data, session)
	consumed = addViewData(data, session, constants.Old) || consumed
	consumed = addViewData(data, session, constants.Flash) || consumed

	if consumed {
		session.Options.Path = "/" // Sikrer samme sti
		session.Save(r, w)
	}

	data[constants.Csrf_token] = r.Context().Value(constants.Csrf_token)
	data["ContentView"] = tmplView
	return data
}

var (
	sessionStoreOnce sync.Once
	sessionStore     *sessions.CookieStore
)

// getSessionStore returns the cookie store for the session, created once
// (the app key is loaded from .env before the first request).
func getSessionStore() *sessions.CookieStore {
	sessionStoreOnce.Do(func() {
		sessionStore = sessions.NewCookieStore([]byte(util.GetEnv(constants.APP_Key, "")))
	})
	return sessionStore
}

// addErrors adds validation errors from the session to the view data. It
// returns true if there were any, so they are removed from the session
// (shown only once, like a flash message).
func addErrors(propVal map[string]any, session *sessions.Session) bool {
	var name = constants.Errors

	if messages, ok := session.Values[name].(string); ok {
		var m2 map[string][]string
		if err := json.Unmarshal([]byte(messages), &m2); err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println("has" + name)
		propVal[name] = m2
		propVal["has"+name] = true

		// Delete the message, so it will not be shown again, after next reload (flash-message)
		delete(session.Values, name)
		return true
	}

	propVal[name] = map[string][]string{}
	propVal["has"+name] = false
	return false
}

// addViewData adds old input or flash messages (name) from the session to the
// view data. It returns true if there were any, so they are removed from the
// session.
func addViewData(propVal map[string]any, session *sessions.Session, name string) bool {
	if messages, ok := session.Values[name].(string); ok {
		var m2 map[string]any
		if err := json.Unmarshal([]byte(messages), &m2); err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println("has" + name)
		propVal[name] = m2
		propVal["has"+name] = true

		// Delete the message, so it will not be shown again, after next reload (flash-message)
		delete(session.Values, name)
		return true
	}

	propVal[name] = map[string]any{}
	propVal["has"+name] = false
	return false
}
