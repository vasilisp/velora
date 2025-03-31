package openai

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/vasilisp/velora/internal/util"
)

const model = openai.ChatModelGPT4oMini

type Client struct {
	client *openai.Client
}

func NewClient(apiKey string) Client {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return Client{client: &client}
}

func extractGPTResponse(chatCompletion *openai.ChatCompletion) (string, error) {
	if chatCompletion == nil {
		return "", fmt.Errorf("nil chatCompletion")
	}
	if len(chatCompletion.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}
	return chatCompletion.Choices[0].Message.Content, nil
}

func (c Client) AskGPT(systemMessage string, userMessage string) (string, error) {
	util.Assert(systemMessage != "", "AskGPT empty systemMessage")
	util.Assert(userMessage != "", "AskGPT empty userMessage")

	chatCompletion, err := c.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemMessage),
			openai.UserMessage(userMessage),
		},
		Model: model,
	})
	if err != nil {
		return "", fmt.Errorf("ChatCompletion error: %v", err)
	}

	return extractGPTResponse(chatCompletion)
}
