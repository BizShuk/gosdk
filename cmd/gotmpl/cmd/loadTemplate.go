package cmd

import (
	"context"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/bizshuk/gosdk/cmd/gotmpl/tmpl"
	"github.com/hairyhenderson/gomplate/v4"
	"github.com/hairyhenderson/gomplate/v4/data"
)

type TemplateLoader struct {
	EventName  string
	Customized bool
	ConfigA    bool
	ConfigB    bool
}

func (t TemplateLoader) Load() {
	fs := tmpl.GetTemplateFiles()

	tmplFs, err := template.New("sample.go.tmpl").Funcs(tmplFuncs).ParseFS(fs, "sample.go.tmpl")
	if err != nil {
		log.Fatalln("Load template failed")
	}

	err = tmplFs.Execute(os.Stdout, t)
	if err != nil {
		log.Fatalln("Gen template failed", err)
	}
}

var (
	tmplFuncs = gomplate.CreateFuncs(context.Background(), &data.Data{})

	tmplFuncSample = template.FuncMap{
		"toLower": strings.ToLower,
		"toUpper": strings.ToUpper,
	}
)
