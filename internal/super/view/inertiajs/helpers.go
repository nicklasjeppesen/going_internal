package inertiajs

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"

	"myapp/internal/super/constants"
	"myapp/internal/super/util"
	"net/http"
	"os"

	struct_to_map "myapp/internal/super/util"

	"github.com/gorilla/sessions"
)

type PageTemplateAssets struct {
	JsFiles  []template.HTMLAttr
	CssFiles []template.HTMLAttr
}

type InertiaInfo struct {
	PageTemplate       *template.Template
	PageTemplateAssets *PageTemplateAssets
	AssetVersion       string
}

type viewparam = map[string]any

// Create a cookie-based session store
var key = util.GetEnv(constants.APP_Key, "")
var store = sessions.NewCookieStore([]byte(key))

func NewPageTemplateAssets() (p *PageTemplateAssets) {
	p = new(PageTemplateAssets)

	if os.Getenv(constants.App_env) == constants.Dev {
		p.JsFiles = append(p.JsFiles,
			template.HTMLAttr("http://localhost:5173/@vite/client"),
			template.HTMLAttr("http://localhost:5173/src/main.tsx"))

		return p
	}
	return p
}

/*
- Getting the page props values
*/
func getPageData(page string, w http.ResponseWriter, r *http.Request, props ...viewparam) map[string]any {

	var propVal = map[string]any{}

	propVal = addViewParams(propVal, w, r, props...)
	propVal = addErrors(propVal, w, r)
	propVal = addCSRFToken(propVal, w, r)

	pageData := map[string]any{
		"component":      page,               //The name of the JavaScript page component.
		"props":          propVal,            // The page props (data).
		"url":            r.URL.String(),     // The page URL.
		"version":        getAssetVersion(r), // The current asset version.
		"encryptHistory": true,               // Whether or not to encrypt the current page's history state.
		"clearHistory":   true,               //Whether or not to clear any encrypted history state.
	}
	return pageData
}

/*
- Returning the pageObject, needed for a first time get to a inertia page
*/
func getPageObject(pageProps map[string]any) map[string]any {

	pageTemplateAssets := NewPageTemplateAssets()
	pageJson, err := json.Marshal(pageProps)
	if err != nil {
		log.Println("Failed to encode page object to JSON", err)
	}
	var data = map[string]any{
		"pageObject":    template.HTMLAttr(string(pageJson)),
		"jsFiles":       pageTemplateAssets.JsFiles,
		"cssFiles":      pageTemplateAssets.CssFiles,
		"isDevelopment": os.Getenv(constants.App_env) == constants.Dev,
	}
	return data
}

/*
- Get the root file path for first time for rendering a inertia page.
- Depends on dev mode or react have been build for prod
*/
func getRootFilePath() string {
	var rootFilePath = ""
	if os.Getenv(constants.App_env) == constants.Dev {
		rootFilePath = "./resources/views/app.html"
	} else {
		rootFilePath = "./public/index.html"
	}
	return rootFilePath
}

/*
- Adding view parameter to the props object.
*/
func addViewParams(propVal map[string]any, w http.ResponseWriter, r *http.Request, props ...viewparam) map[string]any {

	if len(props) > 0 {
		prop := props[0]
		for key, val := range prop {
			propVal[key] = struct_to_map.HasJsonFunc(val)
		}
	}
	return propVal
}

func addErrors(propVal map[string]any, w http.ResponseWriter, r *http.Request) map[string]any {
	// Checking if there is any errors in from previously response
	session, err := store.Get(r, "session-navn")
	if err != nil {
		fmt.Println(err)
	}

	if errormessages, ok := session.Values["errors"].(string); ok {
		var b2 = []byte(errormessages)
		var m2 map[string]string

		err = json.Unmarshal(b2, &m2)
		if err != nil {
			fmt.Println(err.Error())
		}

		propVal["errors"] = m2
		propVal["hasErrors"] = true
		delete(session.Values, "errors")
		session.Save(r, w)
	}
	return propVal
}

func getAssetVersion(r *http.Request) string {
	var assetVersion = "1"
	if _version, ok := r.Context().Value(HEADER_X_INERTIA_VERSION).(string); ok {
		assetVersion = _version
	}
	return assetVersion
}

func inertiaResponse(w http.ResponseWriter, r *http.Request, props map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(HEADER_X_INERTIA, "true")
	json.NewEncoder(w).Encode(props)
}

/*
- Adding CSRFToken to the pros.
*/
func addCSRFToken(_v map[string]any, w http.ResponseWriter, r *http.Request) map[string]any {

	_v[constants.Csrf_token] = r.Context().Value(constants.Csrf_token)
	return _v
}

/*
- return the header status used in redirect and back actions
*/
func getHeader(method string) int {
	if method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete {
		return http.StatusSeeOther
	} else {
		return http.StatusFound
	}
}
