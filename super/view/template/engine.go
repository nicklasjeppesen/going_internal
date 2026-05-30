package template

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/nicklasjeppesen/going_internal/super/constants"
)

type Engine struct {
	templates *template.Template
	funcs     template.FuncMap
}

func New() *Engine {

	engine := &Engine{
		funcs: template.FuncMap{},
	}

	engine.registerCoreFunctions()
	engine.loadTemplates()

	return engine
}

func (e *Engine) loadTemplates() {

	e.templates = template.New("").Funcs(e.funcs)

	filepath.WalkDir("internal/resources/views",
		func(path string, d fs.DirEntry, err error) error {

			if err != nil {
				return err
			}

			if !d.IsDir() && strings.HasSuffix(path, ".template") {
				_, err = e.templates.ParseFiles(path)
				if err != nil {
					return err
				}
			}

			return nil
		})
}

func (e *Engine) registerCoreFunctions() {

	e.funcs["render"] = func(name string, data any) template.HTML {
		return e.RenderTemplate(name, data)
	}

	e.funcs["old"] = func(key string, data map[string]any) any {
		return e.RenderOld(key, data)
	}

	e.funcs["component"] = func(name string, values ...interface{}) (template.HTML, error) {
		return e.RenderComponent(name, values...)
	}

	e.funcs["csrf_token"] = func(data map[string]any) template.HTML {
		return e.renderCsrf_Token(data)
	}

}

func (e *Engine) renderCsrf_Token(data map[string]any) template.HTML {

	token := ""
	if _, exists := data["csrf_token"]; exists == true {
		token = data["csrf_token"].(string)
	}

	return template.HTML(`<input type="hidden" name="csrf_token" value="` +
		template.HTMLEscapeString(token) +
		`">`)

}

func (e *Engine) RenderComponent(name string, args ...any) (template.HTML, error) {
	// 1. Valider (Key + Value)
	if len(args)%2 != 0 {
		return "", fmt.Errorf("RenderComponent '%s' Has an uneven number of argument", name)
	}

	// 2. Build "dict"
	props := make(map[string]interface{}, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			return "", fmt.Errorf("Component %s: key Has to been a string", name)
		}
		props[key] = args[i+1]
	}

	// 3. Render komponenten til en buffer
	var buf bytes.Buffer
	err := e.templates.ExecuteTemplate(&buf, name, props)
	if err != nil {
		return "", err
	}

	// 4. Returner som template.HTML, så Go ikke escaper vores HTML-tags
	return template.HTML(buf.String()), nil
}

func (e *Engine) RenderTemplate(name string, data any) template.HTML {
	var buf bytes.Buffer
	err := e.templates.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return ""
	}

	return template.HTML(buf.String())
}

func (e *Engine) RenderOld(key string, data map[string]any) string {
	if val, exists := data[constants.Old]; exists {
		if oldMap, ok := val.(map[string]string); ok {
			if oldVal, found := oldMap[key]; found {
				return oldVal
			}
		}
	}
	return "" // Return empty string if value does not exists
}
