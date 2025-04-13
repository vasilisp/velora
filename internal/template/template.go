package template

import (
	"bytes"
	"text/template"

	"github.com/vasilisp/velora/internal/data"
	"github.com/vasilisp/velora/internal/util"
)

type Parsed struct {
	tmpl *template.Template
}

func MakeParsed(allTemplates []string) Parsed {
	fsys, err := data.PromptFS()
	if err != nil {
		util.Fatalf("error getting prompt FS: %v\n", err)
	}

	tmpl, err := template.ParseFS(fsys, allTemplates...)
	if err != nil {
		util.Fatalf("error parsing template: %v\n", err)
	}

	util.Assert(tmpl != nil, "MakeParsed nil template")

	return Parsed{tmpl: tmpl}
}

func (p Parsed) Execute(templateName string, d any) (string, error) {
	var prompt bytes.Buffer

	if err := p.tmpl.ExecuteTemplate(&prompt, templateName, d); err != nil {
		util.Fatalf("error executing template: %v\n", err)
	}

	return prompt.String(), nil
}
