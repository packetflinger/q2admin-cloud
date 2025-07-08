package backend

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	llm "github.com/ollama/ollama/api"
)

type LLMConfig struct {
	Model  string
	URL    string
	Stream bool
}

func (c *LLMConfig) Ask(ctx context.Context, prompt string) (string, error) {
	req := &llm.GenerateRequest{
		Model:  "tinyllama:1.1b",
		Prompt: prompt,
		Stream: &c.Stream,
	}
	u, err := url.Parse(c.URL)
	if err != nil {
		return "", fmt.Errorf("error parsing url: %v", err)
	}
	client := llm.NewClient(u, &http.Client{})

	var responses []string
	err = client.Generate(ctx, req, func(resp llm.GenerateResponse) error {
		responses = append(responses, resp.Response)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return strings.Join(responses, " "), nil
}
