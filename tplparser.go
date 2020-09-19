package main

import (
	"fmt"
	"text/template"

	"github.com/alessio/shellescape"
)

func ParseTpl(r string) (*template.Template, error) {
	tpl := template.New("tpl")

	tpl.Funcs(template.FuncMap{
		"bash_escape": func(s interface{}) string {
			return shellescape.Quote(fmt.Sprint(s))
		},
	})

	return tpl.Parse(r)
}
