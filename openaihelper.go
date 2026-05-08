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
	chunkSize   = 1500
	temperature = 0.7
	maxTokens   = 2000
)

// initOpenAI initializes the OpenAI client with the provided API key.
func initOpenAI(apiKey string) (*openai.Client, context.Context, error) {
	if apiKey == "" {
		return nil, nil, fmt.Errorf("API key is empty")
	}
	client := openai.NewClient(apiKey)
	ctx := context.Background()
	return client, ctx, nil
}

// getTokenSize returns the number of tokens in prompt for the given model.
func getTokenSize(model string, prompt string) (int, error) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return 0, fmt.Errorf("failed to get encoding for model: %w", err)
	}

	token := tkm.Encode(prompt, nil, nil)
	return len(token), nil
}

// sliceTextIntoChunks splits the input text into chunks of size chunkSize
// (measured in OpenAI tokens for the given model). Falls back to cl100k_base
// if the model is unknown to tiktoken-go.
func sliceTextIntoChunks(text, model string) ([]string, error) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sanigate: tiktoken doesn't recognize %q, falling back to cl100k_base for chunking\n", model)
		tkm, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, fmt.Errorf("tiktoken fallback failed: %w", err)
		}
	}

	lines := strings.Split(text, "\n")
	chunks := []string{""}
	chunkIndex := 0

	for _, line := range lines {
		ll := len(tkm.Encode(line, nil, nil))
		cl := len(tkm.Encode(chunks[chunkIndex], nil, nil))

		if !(cl+ll < chunkSize) {
			chunkIndex++
			chunks = append(chunks, "")
		}

		chunks[chunkIndex] += line + "\n"
	}

	return chunks, nil
}

// requestCompletion sends a chat completion request for the given model.
func requestCompletion(client *openai.Client, ctx context.Context, prompt, model string) (string, error) {
	completionReq := openai.ChatCompletionRequest{
		Model:            model,
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
