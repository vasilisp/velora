package langchain

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type Client struct {
	llm llms.Model
}

func NewClient(apiKey string) (Client, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithModel("gpt-4"),
	)
	if err != nil {
		return Client{}, fmt.Errorf("error creating OpenAI client: %v", err)
	}
	return Client{llm: llm}, nil
}

func (c *Client) AskGPT(systemMessage string, userMessages []string) (string, error) {
	if systemMessage == "" {
		return "", fmt.Errorf("empty system message")
	}
	if len(userMessages) == 0 {
		return "", fmt.Errorf("empty user messages")
	}

	// Create messages array with system message first
	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextContent{Text: systemMessage}},
		},
	}

	// Add user messages
	for _, msg := range userMessages {
		messages = append(messages, llms.MessageContent{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextContent{Text: msg}},
		})
	}

	// Generate completion
	response, err := c.llm.GenerateContent(context.Background(), messages)
	if err != nil {
		return "", fmt.Errorf("error generating content: %v", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	return response.Choices[0].Content, nil
}
