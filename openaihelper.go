package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"
	openai "github.com/sashabaranov/go-openai"
)

const (
	apiKeyEnvVar = "SNGT_OPENAI_API_KEY"
	chunkSize    = 1500
	temperature  = 0.7
	maxTokens    = 2000
)

// initOpenAI initializes the OpenAI client and returns it along with the context
func initOpenAI() (*openai.Client, context.Context, error) {
	apiKey := os.Getenv(apiKeyEnvVar)
	if apiKey == "" {
		return nil, nil, fmt.Errorf("%s environment variable is not set!", apiKeyEnvVar)
	}

	client := openai.NewClient(apiKey)
	ctx := context.Background()

	return client, ctx, nil
}

// printWarning prints a warning message to STDERR
func getTokenSize(model string, prompt string) (int, error) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return 0, fmt.Errorf("failed to get encoding for model: %w", err)
	}

	token := tkm.Encode(prompt, nil, nil)
	res := len(token)
	return res, nil
}

// sliceTextIntoChunks splits the input text into chunks of size chunkSize
// (measured in OpenAI tokens as defined by the model and the tiktoken library)
func sliceTextIntoChunks(text string) ([]string, error) {
	lines := strings.Split(text, "\n")
	chunks := []string{""}
	chunkIndex := 0

	for _, line := range lines {
		ll, err := getTokenSize(openai.GPT3Dot5Turbo, line)
		if err != nil {
			return nil, fmt.Errorf("failed to get token size of the line: %w", err)
		}
		cl, err := getTokenSize(openai.GPT3Dot5Turbo, chunks[chunkIndex])
		if err != nil {
			return nil, fmt.Errorf("failed to get token size of the chunk: %w", err)
		}

		if !(cl+ll < chunkSize) {
			chunkIndex += 1
			chunks = append(chunks, "")
		}

		chunks[chunkIndex] += line + "\n"
	}

	return chunks, nil
}

// requestCompletion sends a request to OpenAI API and returns the response
func requestCompletion(client *openai.Client, ctx context.Context, prompt string) (string, error) {
	completionReq := openai.ChatCompletionRequest{
		Model:            openai.GPT3Dot5Turbo,
		Messages:         []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: prompt}},
		MaxTokens:        maxTokens,
		Temperature:      temperature,
		TopP:             0.0,
		N:                0,
		Stream:           false,
		Stop:             []string{},
		PresencePenalty:  0.0,
		FrequencyPenalty: 0.0,
		LogitBias:        map[string]int{},
		User:             "",
	}

	resp, err := client.CreateChatCompletion(ctx, completionReq)
	if err != nil {
		return "", fmt.Errorf("completion error: %v", err)
	}

	return resp.Choices[0].Message.Content, nil
}
