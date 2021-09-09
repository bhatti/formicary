package web

import (
	"errors"
	"github.com/labstack/echo/v4"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"plexobject.com/formicary/queen/utils"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// A TemplateRenderer implements keeper, loader and reloader for HTML templates
type TemplateRenderer struct {
	*template.Template                  // root template
	dir                string           // root directory
	ext                string           // extension
	devel              bool             // reload every time
	funcs              template.FuncMap // functions
	loadedAt           time.Time        // loaded at (last loading time)
}

// NewTemplateRenderer creates new TemplateRenderer and loads templates. The dir argument is
// directory to load templates from. The ext argument is extension of
// templates. The devel (if true) turns the TemplateRenderer to reload templates
// every Render if there is a change in the dir.
func NewTemplateRenderer(
	dir string,
	ext string,
	devel bool) (tmpl *TemplateRenderer, err error) {
	// get absolute path
	if dir, err = filepath.Abs(dir); err != nil {
		return
	}

	tmpl = new(TemplateRenderer)
	tmpl.dir = dir
	tmpl.ext = ext
	tmpl.devel = devel

	tmpl.funcs = utils.TemplateFuncs()

	if err = tmpl.Load(); err != nil {
		tmpl = nil // drop for GC
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

	// time point
	t.loadedAt = time.Now()

	// unnamed root template
	var root = template.New("")

	if t.funcs != nil {
		root = root.Funcs(t.funcs)
	}

	var walkFunc = func(
		path string,
		info os.FileInfo,
		err error) (_ error) {
		if err != nil {
			return err
		}

		// skip all except regular files
		// TODO (kostyarin): follow symlinks
		if !info.Mode().IsRegular() {
			return
		}

		// filter by extension
		if filepath.Ext(path) != t.ext {
			return
		}

		// get relative path
		var rel string
		if rel, err = filepath.Rel(t.dir, path); err != nil {
			return err
		}

		// name of a template is its relative path without extension
		rel = strings.TrimSuffix(rel, t.ext)

		// load or reload
		var (
			nt = root.New(rel)
			b  []byte
		)

		if b, err = ioutil.ReadFile(path); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "TemplateRenderer",
				"Path":      path,
				"Error":     err,
			}).Info("Reading failed")
			return err
		}

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component": "TemplateRenderer",
				"Path":      path,
			}).Debugf("loading template")
		}
		_, err = nt.Parse(string(b))
		return err
	}

	if err = filepath.Walk(t.dir, walkFunc); err != nil {
		return
	}

	t.Template = root // set or replace
	return
}

// IsModified lookups directory for changes to
// reload (or not to reload) templates if development
// pin is true.
func (t *TemplateRenderer) IsModified() (yep bool, err error) {

	var errStop = errors.New("stop")

	var walkFunc = func(path string, info os.FileInfo, err error) (_ error) {

		// handle walking error if any
		if err != nil {
			return err
		}

		// skip all except regular files
		// TODO (kostyarin): follow symlinks
		if !info.Mode().IsRegular() {
			return
		}

		// filter by extension
		if filepath.Ext(path) != t.ext {
			return
		}

		if yep = info.ModTime().After(t.loadedAt); yep == true {
			return errStop
		}

		return
	}

	// clear the errStop
	if err = filepath.Walk(t.dir, walkFunc); err == errStop {
		err = nil
	}

	return
}

// Render renders template
func (t *TemplateRenderer) Render(
	w io.Writer,
	name string,
	data interface{}, c echo.Context) (err error) {
	// Add global methods if data is a map
	if viewContext, isMap := data.(map[string]interface{}); isMap {
		viewContext["reverse"] = c.Echo().Reverse
	}
	if t.devel == true {
		var modified bool
		if modified, err = t.IsModified(); err != nil {
			return
		}
		if modified == true {
			if err = t.Load(); err != nil {
				return
			}
		}
	}

	err = t.ExecuteTemplate(w, name, data)
	return
}
