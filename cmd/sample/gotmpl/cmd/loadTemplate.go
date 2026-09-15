package cmd

import (
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/bizshuk/gosdk/cmd/sample/gotmpl/tmpl"
)

type TemplateLoader struct {
	EventName  string
	Customized bool
	ConfigA    bool
	ConfigB    bool
}

func (t TemplateLoader) Load() {
	fs := tmpl.GetTemplateFiles()

	tmplFs, err := template.New("sample.go.tmpl").Funcs(TmplFuncSample).ParseFS(fs, "sample.go.tmpl")
	if err != nil {
		log.Fatalln("Load template failed")
	}

	err = tmplFs.Execute(os.Stdout, t)
	if err != nil {
		log.Fatalln("Gen template failed", err)
	}
}

var (
	TmplFuncSample = template.FuncMap{
		"toLower": strings.ToLower,
		"toUpper": strings.ToUpper,
	}
)
