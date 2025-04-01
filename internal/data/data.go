package data

import (
	"embed"
	"io/fs"
)

//go:embed prompts
var promptFS embed.FS

func PromptFS() (fs.FS, error) {
	return fs.Sub(promptFS, "prompts")
}
