package note

import (
	"fmt"
	"path/filepath"

	"github.com/mickael-menu/zk/core/templ"
	"github.com/mickael-menu/zk/core/zk"
	"github.com/mickael-menu/zk/util/errors"
	"github.com/mickael-menu/zk/util/opt"
	"github.com/mickael-menu/zk/util/paths"
	"github.com/mickael-menu/zk/util/rand"
)

// CreateOpts holds the options to create a new note.
type CreateOpts struct {
	// Parent directory for the new note.
	Dir zk.Dir
	// Title of the note.
	Title opt.String
	// Initial content of the note, which will be injected in the template.
	Content opt.String
}

// Create generates a new note from the given options.
// Returns the path of the newly created note.
func Create(
	opts CreateOpts,
	templateLoader templ.Loader,
) (string, error) {
	wrap := errors.Wrapperf("new note")

	filenameTemplate, err := templateLoader.Load(opts.Dir.Config.FilenameTemplate)
	if err != nil {
		return "", err
	}

	var bodyTemplate templ.Renderer = templ.NullRenderer
	if templatePath := opts.Dir.Config.BodyTemplatePath.Unwrap(); templatePath != "" {
		bodyTemplate, err = templateLoader.LoadFile(templatePath)
		if err != nil {
			return "", wrap(err)
		}
	}

	createdNote, err := create(opts, createDeps{
		filenameTemplate: filenameTemplate,
		bodyTemplate:     bodyTemplate,
		genId:            rand.NewIDGenerator(opts.Dir.Config.IDOptions),
		validatePath:     validatePath,
	})
	if err != nil {
		return "", wrap(err)
	}

	err = paths.WriteString(createdNote.path, createdNote.content)
	if err != nil {
		return "", wrap(err)
	}

	return createdNote.path, nil
}

func validatePath(path string) (bool, error) {
	exists, err := paths.Exists(path)
	return !exists, err
}

type createdNote struct {
	path    string
	content string
}

// renderContext holds the placeholder values which will be expanded in the templates.
type renderContext struct {
	ID           string `handlebars:"id"`
	Title        string
	Content      string
	Dir          string
	Filename     string
	FilenameStem string `handlebars:"filename-stem"`
	Extra        map[string]string
}

type createDeps struct {
	filenameTemplate templ.Renderer
	bodyTemplate     templ.Renderer
	genId            func() string
	validatePath     func(path string) (bool, error)
}

func create(
	opts CreateOpts,
	deps createDeps,
) (*createdNote, error) {
	context := renderContext{
		Title:   opts.Title.OrDefault(opts.Dir.Config.DefaultTitle),
		Content: opts.Content.Unwrap(),
		Dir:     opts.Dir.Name,
		Extra:   opts.Dir.Config.Extra,
	}

	path, context, err := genPath(context, opts.Dir, deps)
	if err != nil {
		return nil, err
	}

	content, err := deps.bodyTemplate.Render(context)
	if err != nil {
		return nil, err
	}

	return &createdNote{path: path, content: content}, nil
}

func genPath(
	context renderContext,
	dir zk.Dir,
	deps createDeps,
) (string, renderContext, error) {
	var path string
	for i := 0; i < 50; i++ {
		context.ID = deps.genId()

		filename, err := deps.filenameTemplate.Render(context)
		if err != nil {
			return "", context, err
		}

		filename = filename + "." + dir.Config.Extension
		path = filepath.Join(dir.Path, filename)
		validPath, err := deps.validatePath(path)
		if err != nil {
			return "", context, err
		} else if validPath {
			context.Filename = filepath.Base(path)
			context.FilenameStem = paths.FilenameStem(path)
			return path, context, nil
		}
	}

	return "", context, fmt.Errorf("%v: note already exists", path)
}