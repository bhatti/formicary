package web

import (
	"errors"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/queen/utils"
)

// A TemplateRenderer implements keeper, loader and reloader for HTML templates
type TemplateRenderer struct {
	*template.Template                  // root template
	fsys               fs.FS           // embedded or os FS (nil = use dir on disk)
	dir                string          // root directory (used when fsys is nil)
	ext                string          // extension
	devel              bool            // reload every time
	funcs              template.FuncMap
	loadedAt           time.Time
}

// NewTemplateRenderer creates a TemplateRenderer that walks the OS filesystem under dir.
// Used in development mode when PublicDir is set explicitly.
func NewTemplateRenderer(dir string, ext string, devel bool) (tmpl *TemplateRenderer, err error) {
	if dir, err = filepath.Abs(dir); err != nil {
		return
	}
	tmpl = &TemplateRenderer{dir: dir, ext: ext, devel: devel}
	tmpl.funcs = utils.TemplateFuncs()
	if err = tmpl.Load(); err != nil {
		return nil, err
	}
	return
}

// NewTemplateRendererFromFS creates a TemplateRenderer that walks an embedded fs.FS.
// The viewsSubDir is the sub-path inside the FS that contains the view files (e.g. "public/views").
func NewTemplateRendererFromFS(fsys fs.FS, viewsSubDir string, ext string) (tmpl *TemplateRenderer, err error) {
	sub, err := fs.Sub(fsys, viewsSubDir)
	if err != nil {
		return nil, err
	}
	tmpl = &TemplateRenderer{fsys: sub, ext: ext, devel: false}
	tmpl.funcs = utils.TemplateFuncs()
	if err = tmpl.Load(); err != nil {
		return nil, err
	}
	return
}

// Funcs sets template functions
func (t *TemplateRenderer) Funcs(funcMap template.FuncMap) {
	t.Template = t.Template.Funcs(funcMap)
	t.funcs = funcMap
}

// Load or reload templates
func (t *TemplateRenderer) Load() (err error) {
	t.loadedAt = time.Now()
	var root = template.New("")
	if t.funcs != nil {
		root = root.Funcs(t.funcs)
	}

	if t.fsys != nil {
		err = t.loadFromFS(root)
	} else {
		err = t.loadFromDir(root)
	}
	if err != nil {
		return
	}
	t.Template = root
	return
}

func (t *TemplateRenderer) loadFromFS(root *template.Template) error {
	return fs.WalkDir(t.fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != t.ext {
			return nil
		}
		name := strings.TrimSuffix(path, t.ext)
		b, err := fs.ReadFile(t.fsys, path)
		if err != nil {
			return err
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithField("Path", path).Debug("loading embedded template")
		}
		_, err = root.New(name).Parse(string(b))
		return err
	})
}

func (t *TemplateRenderer) loadFromDir(root *template.Template) error {
	return filepath.Walk(t.dir, func(path string, info os.FileInfo, err error) (_ error) {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return
		}
		if filepath.Ext(path) != t.ext {
			return
		}
		rel, err := filepath.Rel(t.dir, path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(rel, t.ext)
		b, err := os.ReadFile(path)
		if err != nil {
			logrus.WithFields(logrus.Fields{"Path": path, "Error": err}).Info("reading template failed")
			return err
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithField("Path", path).Debug("loading template")
		}
		_, err = root.New(name).Parse(string(b))
		return err
	})
}

// IsModified checks the OS directory for changes (only used in devel/filesystem mode).
func (t *TemplateRenderer) IsModified() (yep bool, err error) {
	if t.fsys != nil {
		return false, nil
	}
	var errStop = errors.New("stop")
	walkFunc := func(path string, info os.FileInfo, err error) (_ error) {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return
		}
		if filepath.Ext(path) != t.ext {
			return
		}
		if yep = info.ModTime().After(t.loadedAt); yep {
			return errStop
		}
		return
	}
	if err = filepath.Walk(t.dir, walkFunc); err == errStop {
		err = nil
	}
	return
}

// Render renders template
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) (err error) {
	if viewContext, isMap := data.(map[string]interface{}); isMap {
		viewContext["reverse"] = c.Echo().Reverse
	}
	if t.devel {
		modified, modErr := t.IsModified()
		if modErr != nil {
			return modErr
		}
		if modified {
			if err = t.Load(); err != nil {
				return
			}
		}
	}
	return t.ExecuteTemplate(w, name, data)
}
