package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/openai/openai-go"
)

// BatchRequest represents a single request line in the JSONL file
type BatchRequest struct {
	CustomID string      `json:"custom_id"`
	Method   string      `json:"method"`
	URL      string      `json:"url"`
	Body     RequestBody `json:"body"`
}

// RequestBody is the body of a chat completion request
type RequestBody struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ImageContent represents content with an image URL
type ImageContent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL with detail level
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail"`
}

// BatchResponse represents a single response line from the output JSONL
type BatchResponse struct {
	ID       string         `json:"id"`
	CustomID string         `json:"custom_id"`
	Response *ResponseBody  `json:"response,omitempty"`
	Error    *ResponseError `json:"error,omitempty"`
}

// ResponseBody contains the actual response from OpenAI
type ResponseBody struct {
	StatusCode int             `json:"status_code"`
	Body       json.RawMessage `json:"body"`
}

// ResponseError contains error information
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ChatCompletionResponse represents a chat completion response body
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
}

// Choice represents a single choice in the response
type Choice struct {
	Index   int            `json:"index"`
	Message ResponseMessage `json:"message"`
}

// ResponseMessage is the assistant's response message
type ResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client wraps the OpenAI client
type Client struct {
	client openai.Client
}

// NewClient creates a new OpenAI client
func NewClient() *Client {
	return &Client{
		client: openai.NewClient(),
	}
}

// BuildBatchRequest creates a batch request for an image
func BuildBatchRequest(customID, imageURL, model, prompt, detail string) BatchRequest {
	return BatchRequest{
		CustomID: customID,
		Method:   "POST",
		URL:      "/v1/chat/completions",
		Body: RequestBody{
			Model: model,
			Messages: []Message{
				{
					Role: "user",
					Content: []ImageContent{
						{
							Type: "text",
							Text: prompt,
						},
						{
							Type: "image_url",
							ImageURL: &ImageURL{
								URL:    imageURL,
								Detail: detail,
							},
						},
					},
				},
			},
			MaxTokens: 4096,
		},
	}
}

// BuildJSONL creates JSONL content from a list of batch requests
func BuildJSONL(requests []BatchRequest) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for _, req := range requests {
		if err := encoder.Encode(req); err != nil {
			return nil, fmt.Errorf("failed to encode request: %w", err)
		}
	}
	return buf.Bytes(), nil
}

// UploadBatchFile uploads a JSONL file for batch processing
func (c *Client) UploadBatchFile(ctx context.Context, jsonlData []byte) (string, error) {
	file, err := c.client.Files.New(ctx, openai.FileNewParams{
		File:    openai.File(bytes.NewReader(jsonlData), "batch_input.jsonl", "application/jsonl"),
		Purpose: openai.FilePurposeBatch,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	return file.ID, nil
}

// CreateBatch creates a new batch job
func (c *Client) CreateBatch(ctx context.Context, inputFileID string) (*openai.Batch, error) {
	batch, err := c.client.Batches.New(ctx, openai.BatchNewParams{
		InputFileID:      inputFileID,
		Endpoint:         openai.BatchNewParamsEndpointV1ChatCompletions,
		CompletionWindow: openai.BatchNewParamsCompletionWindow24h,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create batch: %w", err)
	}
	return batch, nil
}

// GetBatch retrieves the status of a batch
func (c *Client) GetBatch(ctx context.Context, batchID string) (*openai.Batch, error) {
	batch, err := c.client.Batches.Get(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch: %w", err)
	}
	return batch, nil
}

// DownloadFile downloads a file from OpenAI
func (c *Client) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	resp, err := c.client.Files.Content(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}
	return data, nil
}

// ParseBatchOutput parses the JSONL output from a completed batch
func ParseBatchOutput(data []byte) ([]BatchResponse, error) {
	var responses []BatchResponse
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var resp BatchResponse
		if err := decoder.Decode(&resp); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

// ExtractContent extracts the text content from a batch response
func ExtractContent(resp BatchResponse) (string, error) {
	if resp.Error != nil {
		return "", fmt.Errorf("request failed: %s - %s", resp.Error.Code, resp.Error.Message)
	}
	if resp.Response == nil {
		return "", fmt.Errorf("no response body")
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(resp.Response.Body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}
