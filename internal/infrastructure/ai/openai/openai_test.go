package openai //nolint:testpackage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GenerateContent_Success(t *testing.T) {
	t.Parallel()

	prompt := "describe this image"
	imageBytes := []byte("fake-image-data")
	expectedText := "This is a cat sitting on a table."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return
		}

		var req chatRequest
		err = json.Unmarshal(body, &req)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "gpt-4o-mini", req.Model)
		if !assert.Len(t, req.Messages, 1) {
			return
		}

		msg := req.Messages[0]
		assert.Equal(t, "user", msg.Role)
		if !assert.Len(t, msg.Content, 2) {
			return
		}

		assert.Equal(t, "text", msg.Content[0].Type)
		assert.Equal(t, prompt, msg.Content[0].Text)

		assert.Equal(t, "image_url", msg.Content[1].Type)
		if !assert.NotNil(t, msg.Content[1].ImageURL) {
			return
		}
		expectedURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imageBytes)
		assert.Equal(t, expectedURL, msg.Content[1].ImageURL.URL)

		resp := chatResponse{
			Choices: []choice{
				{
					Message: responseMessage{
						Content: expectedText,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp) //nolint
	}))
	defer server.Close()

	client := &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cfg:        config{APIKey: "test-key"},
		baseURL:    server.URL,
	}

	result, err := client.GenerateContent(context.Background(), prompt, imageBytes)

	require.NoError(t, err)
	assert.Equal(t, expectedText, result)
}

func TestClient_GenerateContent_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`)) //nolint
	}))
	defer server.Close()

	client := &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cfg:        config{APIKey: "test-key"},
		baseURL:    server.URL,
	}

	result, err := client.GenerateContent(context.Background(), "prompt", []byte("image"))

	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error (status 500)")
}

func TestClient_GenerateContent_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json at all`)) //nolint
	}))
	defer server.Close()

	client := &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cfg:        config{APIKey: "test-key"},
		baseURL:    server.URL,
	}

	result, err := client.GenerateContent(context.Background(), "prompt", []byte("image"))

	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

func TestClient_GenerateContent_EmptyChoices(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []choice{},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp) //nolint
	}))
	defer server.Close()

	client := &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cfg:        config{APIKey: "test-key"},
		baseURL:    server.URL,
	}

	result, err := client.GenerateContent(context.Background(), "prompt", []byte("image"))

	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no choices in response")
}

func TestClient_GenerateContent_EmptyContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []choice{
				{
					Message: responseMessage{
						Content: "",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp) //nolint
	}))
	defer server.Close()

	client := &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cfg:        config{APIKey: "test-key"},
		baseURL:    server.URL,
	}

	result, err := client.GenerateContent(context.Background(), "prompt", []byte("image"))

	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty content in response")
}

func TestClient_GenerateContent_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient: &http.Client{Timeout: 100 * time.Millisecond},
		cfg:        config{APIKey: "test-key"},
		baseURL:    server.URL,
	}

	result, err := client.GenerateContent(context.Background(), "prompt", []byte("image"))

	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send request")
}

func TestNew_Success(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-api-key")
	t.Setenv("OPENAI_TIMEOUT", "10s")

	client, err := New()

	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "test-api-key", client.cfg.APIKey)
	assert.Equal(t, 10*time.Second, client.cfg.Timeout)
	assert.Equal(t, defaultBaseURL, client.baseURL)
	assert.NotNil(t, client.httpClient)
}

func TestNew_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "placeholder")

	require.NoError(t, os.Unsetenv("OPENAI_API_KEY"))

	client, err := New()

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to parse config")
}
