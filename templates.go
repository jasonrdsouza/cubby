package main

import (
	_ "embed"
	"text/template"
)

//go:embed index.html
var indexFile string

func IndexTemplate() *template.Template {
	indexTemplate, err := template.New("index").Parse(indexFile)
	if err != nil {
		panic(err)
	}
	return indexTemplate
}
