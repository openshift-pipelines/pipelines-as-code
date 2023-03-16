package reconciler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jedib0t/go-pretty/v6/text"
	gpt3 "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

type ChatGPT struct {
	gptClient *gpt3.Client
	logger    *zap.SugaredLogger
}

func NewCopilot(log *zap.SugaredLogger) *ChatGPT {
	apiKey := os.Getenv("CHATGPT_API_KEY")
	if apiKey == "" {
		return nil
	}
	log.Info("ChatGPT API key found, enabling copilot analyze on PRs")
	return &ChatGPT{logger: log, gptClient: gpt3.NewClient(apiKey)}
}

// adapted from https://github.com/kkdai/chatgpt/blob/master/main.go
func (c *ChatGPT) GetResponse(ctx context.Context, question string) (string, error) {
	ret := ""
	req := gpt3.CompletionRequest{
		Model:     gpt3.GPT3TextDavinci001,
		MaxTokens: 300,
		Prompt:    question,
		Stream:    true,
	}

	resp, err := c.gptClient.CreateCompletionStream(ctx, req)
	if err != nil {
		return "", fmt.Errorf("CreateCompletionStream returned error: %w", err)
	}
	defer resp.Close()

	counter := 0
	for {
		data, err := resp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("robot error: ChatGPT stream error: %w", err)
		}
		counter++
		ret += data.Choices[0].Text
	}
	if counter == 0 {
		return "", fmt.Errorf("robot error: ChatGPT Stream did not return any responses")
	}
	// justify string adding some \newline to fit 20 chars

	return text.WrapSoft(ret, 80), nil
}
