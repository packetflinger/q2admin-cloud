package backend

import (
	"context"
	"testing"
)

func TestLlmAsk(t *testing.T) {
	c := LLMConfig{
		// Model:  "tinyllama:1.1b",
		Model:  "deepseek-r1:80b",
		URL:    "http://srv2.joereid.com:11434",
		Stream: false,
	}

	tests := []struct {
		name   string
		prompt string
		config LLMConfig
	}{
		{
			name:   "test1",
			prompt: "what should I write in my mom's birthday card?",
			config: c,
		},
	}

	ctx := context.Background()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			answer, err := tc.config.Ask(ctx, tc.prompt)
			if err != nil {
				t.Error(err)
			}
			t.Error(answer)
		})
	}
}
