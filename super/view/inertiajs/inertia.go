package inertiajs

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
)

type Inertia struct {
}

// redirect to a current url-view
func (inertia Inertia) RedirectTo(url string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Vary", HEADER_X_INERTIA)
		w.Header().Set("Location", url)
		w.Header().Set("referer", r.Header.Get("referer"))
		w.WriteHeader(getHeader(r.Method))
	}
}

// redirect back to the current view
func (Inertia Inertia) Back(errors ...map[string]string) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		if errors[0] != nil {
			session, _ := store.Get(r, "session-navn")
			encodedErrors, _ := json.Marshal(errors[0])
			session.Values["errors"] = string(encodedErrors)
			session.Save(r, w) // Husk at gemme
		}

		// Set headers
		w.Header().Set("Vary", HEADER_X_INERTIA)
		w.Header().Set("Location", r.URL.String())
		w.Header().Set("referer", r.Header.Get("referer"))
		w.WriteHeader(getHeader(r.Method))
	}
}

/*
- Returning a inertia View
*/
func (inertia Inertia) View(page string, props ...viewparam) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		pageProps := getPageData(page, w, r, props...)
		if r.Header.Get(HEADER_X_INERTIA) == "true" {
			inertiaResponse(w, r, pageProps)
			return
		}

		pageObject := getPageObject(pageProps)
		tmpl, err := template.ParseFiles(getRootFilePath())
		if err != nil {
			http.Error(w, "Unable to load template", http.StatusInternalServerError)
			log.Println("Template error:", err)
			return
		}

		err = tmpl.Execute(w, pageObject)
		if err != nil {
			http.Error(w, "Unable to render template", http.StatusInternalServerError)
			log.Println("Execute error:", err)
		}
	}
}
