package main

import "text/template"

func IndexTemplate() *template.Template {
	indexTemplate, err := template.ParseFiles("index.html")
	if err != nil {
		panic(err)
	}
	return indexTemplate
}
