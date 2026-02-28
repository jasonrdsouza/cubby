package main

import (
	_ "embed"
	htmltemplate "html/template"
	"text/template"
)

//go:embed index.html
var indexFile string

//go:embed js-client/cubby-client.js
var clientJS string

//go:embed viewer.html
var viewerFile string

func IndexTemplate() *template.Template {
	indexTemplate, err := template.New("index").Parse(indexFile)
	if err != nil {
		panic(err)
	}
	return indexTemplate
}

func ViewerTemplate() *htmltemplate.Template {
	viewerTemplate, err := htmltemplate.New("viewer").Parse(viewerFile)
	if err != nil {
		panic(err)
	}
	return viewerTemplate
}
