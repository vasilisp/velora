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
		openai.WithModel("gpt-4o"),
	)
	if err != nil {
		return Client{}, fmt.Errorf("error creating OpenAI client: %v", err)
	}
	return Client{llm: llm}, nil
}

func (c *Client) AskGPT(systemMessage string, userMessages []string) (string, error) {
	if len(userMessages) == 0 {
		return "", fmt.Errorf("empty user messages")
	}

	var messages []llms.MessageContent
	if systemMessage != "" {
		messages = make([]llms.MessageContent, 1, len(userMessages)+1)
		messages[0] = llms.MessageContent{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextContent{Text: systemMessage}},
		}
	} else {
		messages = make([]llms.MessageContent, 0, len(userMessages))
	}

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
