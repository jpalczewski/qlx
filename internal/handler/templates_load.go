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
// It panics on a startup-time template error, including duplicate page names
// and pages whose define does not match their file name.
func LoadTemplates(fns TemplateFuncs) map[string]*template.Template {
	layoutTmpl := loadLayout(fns)
	mergeSharedTemplateDirs(layoutTmpl)
	pages, err := discoverPages(embedded.Templates, "templates/pages", layoutTmpl)
	if err != nil {
		panic(err)
	}
	return pages
}

// mergeSharedTemplateDirs auto-discovers top-level subdirectories under templates/
// (excluding pages and layouts) and merges all .html files into tmpl.
func mergeSharedTemplateDirs(tmpl *template.Template) {
	entries, err := fs.ReadDir(embedded.Templates, "templates")
	if err != nil {
		panic(err)
	}
	skip := map[string]bool{"pages": true, "layouts": true}
	for _, e := range entries {
		if !e.IsDir() || skip[e.Name()] {
			continue
		}
		mergeHTMLDir(tmpl, "templates/"+e.Name())
	}
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

// pageTemplateName derives the template name (and map key) of a page file from
// its basename: extension stripped, underscores turned into dashes. Each page
// file must {{ define }} a template with exactly this name.
func pageTemplateName(fpath string) string {
	return strings.ReplaceAll(strings.TrimSuffix(path.Base(fpath), ".html"), "_", "-")
}

// discoverPages walks root in fsys and registers each .html file as a named page
// template keyed by pageTemplateName. It fails fast, returning an error, when two
// files map to the same key (naming both paths) or when a file does not define a
// template matching its key.
func discoverPages(fsys fs.FS, root string, layoutTmpl *template.Template) (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template)
	sources := make(map[string]string) // key -> file path that registered it
	err := fs.WalkDir(fsys, root, func(fpath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || path.Ext(fpath) != ".html" {
			return walkErr
		}
		name := pageTemplateName(fpath)
		if prev, dup := sources[name]; dup {
			return fmt.Errorf("duplicate page template name %q: %s and %s", name, prev, fpath)
		}
		content, err := fs.ReadFile(fsys, fpath)
		if err != nil {
			return err
		}
		tmpl, err := layoutTmpl.Clone()
		if err != nil {
			return fmt.Errorf("clone layout for %s: %w", fpath, err)
		}
		// The layout clone already holds shared partials, so only a template
		// that this file itself (re)defines counts as satisfying the key.
		before := tmpl.Lookup(name)
		if tmpl, err = tmpl.Parse(string(content)); err != nil {
			return fmt.Errorf("parse %s: %w", fpath, err)
		}
		after := tmpl.Lookup(name)
		if after == nil || (before != nil && after.Tree == before.Tree) {
			return fmt.Errorf("page %s must define template %q (derived from its file name)", fpath, name)
		}
		wrapper := `{{ define "content" }}{{ template "` + name + `" . }}{{ end }}`
		if tmpl, err = tmpl.Parse(wrapper); err != nil {
			return fmt.Errorf("parse content wrapper for %s: %w", fpath, err)
		}
		templates[name] = tmpl
		sources[name] = fpath
		return nil
	})
	if err != nil {
		return nil, err
	}
	return templates, nil
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
