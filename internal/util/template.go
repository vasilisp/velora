package util

import (
	"bytes"
	"text/template"

	"github.com/vasilisp/velora/internal/data"
)

func ExecuteTemplate(templateName string, allTemplates []string, d any) (string, error) {
	fsys, err := data.PromptFS()
	if err != nil {
		Fatalf("error getting prompt FS: %v\n", err)
	}

	t, err := template.ParseFS(fsys, allTemplates...)
	if err != nil {
		Fatalf("error parsing template: %v\n", err)
	}

	var systemPrompt bytes.Buffer
	if err := t.ExecuteTemplate(&systemPrompt, templateName, d); err != nil {
		Fatalf("error executing template: %v\n", err)
	}

	return systemPrompt.String(), nil
}
