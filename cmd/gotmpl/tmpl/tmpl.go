package tmpl

import (
	"embed"
)

//go:embed *.tmpl
var templateFs embed.FS

func GetTemplateFiles() embed.FS {
	return templateFs
}
