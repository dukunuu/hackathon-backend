// package ai
package ai

import (
	"bytes"      // For creating a reader from the JSON payload
	"context"    // For request contexts, good practice
	"encoding/json" // For JSON marshalling and unmarshalling
	"fmt"
	"io"       // For reading the response body
	"net/http" // For making HTTP requests
	"net/url"  // For joining URL paths
	"time"     // For HTTP client timeout

	"github.com/dukunuu/hackathon_backend/config" // Assuming this path is correct
)

// OllamaRequestPayload defines the structure for the /api/generate request.
type OllamaRequestPayload struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	// Options map[string]interface{} `json:"options,omitempty"` // Optional
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaResponse struct {
	Model           string        `json:"model"`
	CreatedAt       time.Time     `json:"created_at"`
	Message         OllamaMessage `json:"message"` // For chat models
	Response        string        `json:"response"` // For older/completion models
	Done            bool          `json:"done"`
	TotalDuration   int64         `json:"total_duration"`
	LoadDuration    int64         `json:"load_duration"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
	EvalDuration    int64         `json:"eval_duration"`
	// Context         []int         `json:"context,omitempty"` // Optional
}

type OllamaModel struct {
	modelName   string
	systemPrompt string
	ollamaAddr  string // Base address of the running Ollama instance (e.g., "http://localhost:11434")
	httpClient  *http.Client
}

func NewOllamaModel(addr string, modelName string, systemPrompt string) (*OllamaModel, error) {
	_, err := url.ParseRequestURI(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid Ollama address URL '%s': %w", addr, err)
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second, // Set a reasonable timeout
	}

	healthCheckURL := addr
	if _, err := url.Parse(healthCheckURL); err == nil { // Ensure it's a valid base URL
		req, err := http.NewRequestWithContext(context.Background(), "GET", healthCheckURL, nil)
		if err == nil {
			resp, err := httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("ollama instance at %s is not reachable: %w", addr, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				return nil, fmt.Errorf("ollama instance at %s returned status %d; expected 200. Body: %s", addr, resp.StatusCode, string(bodyBytes))
			}
		} else {
            // Log or handle error creating health check request
            fmt.Printf("Warning: Could not create health check request for %s: %v\n", addr, err)
        }
	}


	return &OllamaModel{
		modelName:   modelName,
		systemPrompt: systemPrompt,
		ollamaAddr:  addr,
		httpClient:  httpClient,
	}, nil
}

// Init initializes the AI package using plain HTTP.
func Init(cfg *config.Config) (*OllamaModel, error) {
	ollamaAddr := cfg.OLLAMA_ADDR
	ollamaModelName := cfg.OLLAMA_MODEL_NAME
	ollamaSystemPrompt := cfg.OLLAMA_SYSTEM_PROMPT

	if ollamaAddr == "" {
		return nil, fmt.Errorf("OLLAMA_ADDR is not set in configuration")
	}
	if ollamaModelName == "" {
		return nil, fmt.Errorf("OLLAMA_MODEL_NAME is not set in configuration")
	}

	return NewOllamaModel(ollamaAddr, ollamaModelName, ollamaSystemPrompt)
}

// GenerateResponse sends a request to the Ollama /api/generate endpoint using plain HTTP.
func (om *OllamaModel) GenerateResponse(userPrompt string) (string, error) {
	// Construct the target URL for the /api/generate endpoint
	targetURL, err := url.JoinPath(om.ollamaAddr, "/api/generate")
	if err != nil {
		return "", fmt.Errorf("failed to create target URL: %w", err)
	}

	// Prepare the request payload
	payload := OllamaRequestPayload{
		Model: om.modelName,
		Messages: []OllamaMessage{
			{Role: "system", Content: om.systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false, // For a single, non-streaming response
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		context.Background(), // Or a more specific context
		"POST",
		targetURL,
		bytes.NewBuffer(payloadBytes),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := om.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Unmarshal the JSON response
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(bodyBytes, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal Ollama response: %w. Body: %s", err, string(bodyBytes))
	}

	if ollamaResp.Message.Role == "assistant" && ollamaResp.Message.Content != "" {
		return ollamaResp.Message.Content, nil
	}
	if ollamaResp.Response != "" {
		return ollamaResp.Response, nil
	}


	return "", fmt.Errorf("no assistant response content found in Ollama response: %s", string(bodyBytes))
}

