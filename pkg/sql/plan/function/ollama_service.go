package function

import (
	"bytes"
	"encoding/json"
	"github.com/matrixorigin/matrixone/pkg/common/moerr"
	"io"
	"net/http"
)

type OllamaSingleEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type OllamaMultipleEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type OllamaEmbeddingResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float32 `json:"embeddings"`
	TotalDuration   int64       `json:"total_duration"`
	LoadDuration    int64       `json:"load_duration"`
	PromptEvalCount int         `json:"prompt_eval_count"`
}

type OllamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaGenerateResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Prepare & send the HTTP request, read the response body, return embeddings
func callOllamaService(requestBody []byte, proxy string) ([][]float32, error) {
	// Prepare & send the HTTP request
	req, err := http.NewRequest("POST", proxy, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, moerr.NewInvalidInputNoCtxf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, moerr.NewInvalidInputNoCtxf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, moerr.NewInvalidInputNoCtxf("received non-200 response: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, moerr.NewInvalidInputNoCtxf("failed to read response body: %v", err)
	}

	var embeddingResponse OllamaEmbeddingResponse
	err = json.Unmarshal(body, &embeddingResponse)
	if err != nil {
		return nil, moerr.NewInvalidInputNoCtxf("failed to unmarshal response body: %v", err)
	}

	return embeddingResponse.Embeddings, nil
}

// take single input, make a POST request to Ollama API and return embedding
func getOllamaSingleEmbedding(input string, model string, proxy string) ([]float32, error) {
	payload := OllamaSingleEmbeddingRequest{
		Model: model,
		Input: input,
	}

	// Marshal the payload to JSON
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return nil, moerr.NewInvalidInputNoCtxf("failed to marshal request body: %v", err)
	}

	embeddings, err := callOllamaService(requestBody, proxy)

	return embeddings[0], nil
}

// Prepare & send the HTTP request, read the response body, and return the generated response
func callOllamaGenerateService(requestBody []byte, proxy string) (string, error) {
	// Prepare & send the HTTP request
	req, err := http.NewRequest("POST", proxy, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", moerr.NewInvalidInputNoCtxf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", moerr.NewInvalidInputNoCtxf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", moerr.NewInvalidInputNoCtxf("received non-200 response: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", moerr.NewInvalidInputNoCtxf("failed to read response body: %v", err)
	}

	var generateResponse OllamaGenerateResponse
	err = json.Unmarshal(body, &generateResponse)
	if err != nil {
		return "", moerr.NewInvalidInputNoCtxf("failed to unmarshal response body: %v", err)
	}

	return generateResponse.Response, nil
}

// take a prompt, make a POST request to Ollama API and return the generated response
func getOllamaGeneratedResponse(prompt string, model string, proxy string) (string, error) {
	payload := OllamaGenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false, // No streaming, generate one complete response
	}

	// Marshal the payload to JSON
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return "", moerr.NewInvalidInputNoCtxf("failed to marshal request body: %v", err)
	}

	// Call the Ollama service
	response, err := callOllamaGenerateService(requestBody, proxy)
	if err != nil {
		return "", err
	}

	return response, nil
}
