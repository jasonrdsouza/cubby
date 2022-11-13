package main

import "text/template"

func IndexTemplate() *template.Template {
	const keyListingTemplate = `
<!DOCTYPE html>
<html>
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Cubby Server</title>
    <style>
    body{
      max-width:650px;
      margin:40px auto;
      padding:0 10px;
      font:18px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji";
      color:#444
    }
    h1,h2,h3{
      line-height:1.2
    }
    @media (prefers-color-scheme: dark){
      body{
        color:#c9d1d9;
        background:#0d1117
      }
      a:link{
        color:#58a6ff
      }
      a:visited{
        color:#8e96f0
      }
    }
    </style>
  </head>
  <body>
    <h1>Occupied Cubbies</h1>
    <ul>
    {{range .}}
        <li><a href="{{.}}">{{.}}</a></li>
    {{else}}
      <div><strong>No entries</strong></div>
    {{end}}
    </ul>
  </body>
</html>`

	return template.Must(template.New("index").Parse(keyListingTemplate))
}
