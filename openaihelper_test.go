package main

// Test openaihelper.go
import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// NOTE: Not testing OpenAI yet die to https://github.com/sashabaranov/go-openai/issues/30

func Test_getTokenSize(t *testing.T) {
	model := openai.GPT3Dot5Turbo
	prompt := "This is a test prompt."
	expected := 6
	actual, err := getTokenSize(model, prompt)
	if err != nil {
		t.Errorf("getTokenSize(%s, %s) failed: %s", model, prompt, err)
	}
	if actual != expected {
		t.Errorf("getTokenSize(%s, %s) expected %d, got %d", model, prompt, expected, actual)
	}
}

func Test_sliceTextIntoChunks(t *testing.T) {
	text := "This is a test prompt."
	expected := []string{"This is a test prompt.\n"}
	actual, err := sliceTextIntoChunks(text)
	if err != nil {
		t.Errorf("sliceTextIntoChunks(%s) failed: %s", text, err)
	}
	if len(actual) != len(expected) {
		t.Errorf("sliceTextIntoChunks(%s) expected %d chunks, got %d", text, len(expected), len(actual))
	}
	for i := range actual {
		if actual[i] != expected[i] {
			t.Errorf("sliceTextIntoChunks(%s) expected chunk %d to be %s, got %s", text, i, expected[i], actual[i])
		}
	}
}
