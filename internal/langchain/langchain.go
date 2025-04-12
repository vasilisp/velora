package langchain

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/vasilisp/velora/internal/util"
)

type Client struct {
	llm llms.Model
}

type MessageType string

const (
	MessageTypeSystem MessageType = "system"
	MessageTypeHuman  MessageType = "human"
	MessageTypeAI     MessageType = "ai"
)

func (m MessageType) ToLangchainMessageType() llms.ChatMessageType {
	switch m {
	case MessageTypeSystem:
		return llms.ChatMessageTypeSystem
	case MessageTypeHuman:
		return llms.ChatMessageTypeHuman
	case MessageTypeAI:
		return llms.ChatMessageTypeAI
	}
	util.Fatalf("invalid message type: %s", m)
	return llms.ChatMessageTypeHuman
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

type Message struct {
	Role    MessageType
	Content string
}

func (m Message) ToLangchainMessage() llms.MessageContent {
	return llms.MessageContent{
		Role:  m.Role.ToLangchainMessageType(),
		Parts: []llms.ContentPart{llms.TextContent{Text: m.Content}},
	}
}

func (c *Client) AskGPT(messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("empty user messages")
	}

	messagesLangchain := make([]llms.MessageContent, len(messages))
	for i, msg := range messages {
		messagesLangchain[i] = msg.ToLangchainMessage()
	}

	// Generate completion
	response, err := c.llm.GenerateContent(context.Background(), messagesLangchain, llms.WithTemperature(0.2))
	if err != nil {
		return "", fmt.Errorf("error generating content: %v", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	return response.Choices[0].Content, nil
}
