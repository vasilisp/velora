package template

import (
	"bytes"
	"text/template"

	"github.com/vasilisp/velora/internal/data"
	"github.com/vasilisp/velora/internal/util"
)

type Parsed interface {
	Execute(templateName string, d any) (string, error)
	Has(name string) bool
}

type parsed struct {
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

	return &parsed{tmpl: tmpl}
}

func (p *parsed) Execute(templateName string, d any) (string, error) {
	var prompt bytes.Buffer

	if err := p.tmpl.ExecuteTemplate(&prompt, templateName, d); err != nil {
		util.Fatalf("error executing template: %v\n", err)
	}

	return prompt.String(), nil
}

func (p *parsed) Has(name string) bool {
	return p.tmpl.Lookup(name) != nil
}
