package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/erxyi/qlx/internal/embedded"
	"github.com/erxyi/qlx/internal/shared/palette"
	"github.com/erxyi/qlx/internal/store"
)

// TemplateFuncs holds tag-related functions injected into templates.
type TemplateFuncs struct {
	ResolveTags func([]string) []store.Tag
	GetTag      func(string) *store.Tag
}

// LoadTemplates discovers and parses all HTML templates from the embedded FS.
func LoadTemplates(fns TemplateFuncs) map[string]*template.Template {
	layoutTmpl := loadLayout(fns)
	mergeHTMLDir(layoutTmpl, "templates/partials")
	mergeHTMLDir(layoutTmpl, "templates/components")
	return discoverPages(layoutTmpl)
}

func loadLayout(fns TemplateFuncs) *template.Template {
	content, err := embedded.Templates.ReadFile("templates/layouts/base.html")
	if err != nil {
		panic(err)
	}
	return template.Must(template.New("layout").Funcs(template.FuncMap{
		"dict":        dict,
		"resolveTags": fns.ResolveTags,
		"tagParentName": func(parentID string) string {
			if parentID == "" {
				return ""
			}
			if t := fns.GetTag(parentID); t != nil {
				return t.Name
			}
			return ""
		},
		"icon": func(name string) template.HTML {
			data, err := palette.SVG(name)
			if err != nil {
				return ""
			}
			return template.HTML(data) //nolint:gosec
		},
		"paletteHex": func(name string) string {
			c, ok := palette.ColorByName(name)
			if !ok {
				return ""
			}
			return c.Hex
		},
		"allColors":      palette.AllColors,
		"iconCategories": palette.IconCategories,
	}).Parse(string(content)))
}

// mergeHTMLDir walks a directory and parses all .html files into tmpl.
func mergeHTMLDir(tmpl *template.Template, dir string) {
	err := fs.WalkDir(embedded.Templates, dir, func(fpath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || path.Ext(fpath) != ".html" {
			return walkErr
		}
		content, err := embedded.Templates.ReadFile(fpath)
		if err != nil {
			return err
		}
		template.Must(tmpl.Parse(string(content)))
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// discoverPages walks templates/pages/ and registers each .html as a named template.
func discoverPages(layoutTmpl *template.Template) map[string]*template.Template {
	templates := make(map[string]*template.Template)
	err := fs.WalkDir(embedded.Templates, "templates/pages", func(fpath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || path.Ext(fpath) != ".html" {
			return walkErr
		}
		name := strings.ReplaceAll(strings.TrimSuffix(path.Base(fpath), ".html"), "_", "-")
		content, err := embedded.Templates.ReadFile(fpath)
		if err != nil {
			return err
		}
		tmpl, err := layoutTmpl.Clone()
		if err != nil {
			return err
		}
		tmpl = template.Must(tmpl.Parse(string(content)))
		wrapper := `{{ define "content" }}{{ template "` + name + `" . }}{{ end }}`
		tmpl = template.Must(tmpl.Parse(wrapper))
		templates[name] = tmpl
		return nil
	})
	if err != nil {
		panic(err)
	}
	return templates
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict expects an even number of arguments")
	}

	result := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		result[key] = values[i+1]
	}

	return result, nil
}
