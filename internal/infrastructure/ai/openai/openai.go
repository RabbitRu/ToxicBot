package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.openai.com"

const maxRetries = 3

type Client struct {
	httpClient *http.Client
	cfg        config
	baseURL    string
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message responseMessage `json:"message"`
}

type responseMessage struct {
	Content string `json:"content"`
}

func New() (*Client, error) {
	client := &Client{
		baseURL: defaultBaseURL,
	}

	if err := client.parseConfig(); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	client.httpClient = &http.Client{
		Timeout: client.cfg.Timeout,
	}

	return client, nil
}

func (c *Client) GenerateContent(
	ctx context.Context,
	prompt string,
	imageBytes []byte,
) (string, error) {
	req := chatRequest{
		Model: "gpt-4o-mini",
		Messages: []message{
			{
				Role: "user",
				Content: []contentPart{
					{Type: "text", Text: prompt},
					{
						Type: "image_url",
						ImageURL: &imageURL{
							URL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(
								imageBytes,
							),
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	var resp *http.Response
	for i := 0; i <= maxRetries; i++ {
		resp, err = c.httpClient.Do(httpReq)
		if err == nil && resp.StatusCode < 500 {
			break
		}

		if i < maxRetries {
			if resp != nil {
				resp.Body.Close() //nolint
			}

			time.Sleep(time.Duration(i+1) * time.Second)

			httpReq, err = http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				url,
				bytes.NewBuffer(jsonData),
			)
			if err != nil {
				return "", fmt.Errorf("failed to create request: %w", err)
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to send request after retries: %w", err)
	}
	defer resp.Body.Close() //nolint

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", errors.New("no choices in response")
	}

	content := chatResp.Choices[0].Message.Content
	if content == "" {
		return "", errors.New("empty content in response")
	}

	return content, nil
}
