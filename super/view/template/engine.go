package template

import (
	"bytes"
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
