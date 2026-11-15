package handler

import (
	"html/template"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/erxyi/qlx/internal/embedded"
)

func page(define string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(`{{ define "` + define + `" }}hello {{ . }}{{ end }}`)}
}

func TestDiscoverPages(t *testing.T) {
	tests := []struct {
		name     string
		fsys     fstest.MapFS
		wantKeys []string
		wantErr  []string // substrings the error must contain
	}{
		{
			name: "happy path",
			fsys: fstest.MapFS{
				"pages/a/one.html":       page("one"),
				"pages/b/two_words.html": page("two-words"),
				"pages/b/readme.txt":     {Data: []byte("ignored")},
			},
			wantKeys: []string{"one", "two-words"},
		},
		{
			name: "duplicate basename in two directories",
			fsys: fstest.MapFS{
				"pages/a/dup.html": page("dup"),
				"pages/b/dup.html": page("dup"),
			},
			wantErr: []string{"pages/a/dup.html", "pages/b/dup.html", `"dup"`},
		},
		{
			name: "underscore and dash basenames collide",
			fsys: fstest.MapFS{
				"pages/a/x_y.html": page("x-y"),
				"pages/b/x-y.html": page("x-y"),
			},
			wantErr: []string{"pages/a/x_y.html", "pages/b/x-y.html"},
		},
		{
			name: "define does not match key",
			fsys: fstest.MapFS{
				"pages/a/one.html": page("other"),
			},
			wantErr: []string{"pages/a/one.html", `"one"`},
		},
		{
			name: "wrong define while a shared partial has the key name",
			fsys: fstest.MapFS{
				"pages/a/shared.html": page("other"),
			},
			wantErr: []string{"pages/a/shared.html", `"shared"`},
		},
		{
			name: "invalid template syntax",
			fsys: fstest.MapFS{
				"pages/a/bad.html": {Data: []byte(`{{ define "bad" }}{{ end`)},
			},
			wantErr: []string{"pages/a/bad.html"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := template.Must(template.New("layout").Parse(
				`{{ block "content" . }}{{ end }}{{ define "shared" }}partial{{ end }}`))
			got, err := discoverPages(tt.fsys, "pages", layout)
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatalf("expected error, got keys %v", got)
				}
				for _, s := range tt.wantErr {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("error %q does not contain %q", err, s)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.wantKeys) {
				t.Fatalf("got %d pages, want %v", len(got), tt.wantKeys)
			}
			for _, k := range tt.wantKeys {
				tm, ok := got[k]
				if !ok {
					t.Fatalf("missing key %q", k)
				}
				var sb strings.Builder
				if err := tm.ExecuteTemplate(&sb, "content", "x"); err != nil || sb.String() != "hello x" {
					t.Errorf("render %q = %q, %v", k, sb.String(), err)
				}
			}
		})
	}
}

func TestLoadTemplates_EmbeddedFS(t *testing.T) {
	pages := LoadTemplates(TemplateFuncs{})
	if len(pages) == 0 {
		t.Fatal("expected embedded pages to load")
	}
	want := map[string]string{}
	err := fs.WalkDir(embedded.Templates, "templates/pages", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || path.Ext(p) != ".html" {
			return walkErr
		}
		want[pageTemplateName(p)] = p
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for k, p := range want {
		if pages[k] == nil {
			t.Errorf("page file %s missing from loaded templates (key %q)", p, k)
		}
	}
	if len(pages) != len(want) {
		t.Errorf("loaded %d pages, found %d page files", len(pages), len(want))
	}
}
