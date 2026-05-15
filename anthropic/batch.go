// Package anthropic wraps the Anthropic Message Batches API for use in marksDAM.
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

// ImageInput is the input to BuildRequests.
type ImageInput struct {
	CustomID string
	ImageURL string
}

// BatchResult is a single parsed result from a completed batch.
type BatchResult struct {
	CustomID string
	Content  string // extracted text; empty on error
	Err      error  // non-nil for errored or expired results
	RawJSON  []byte // full result JSON for storage
}

// Client wraps the Anthropic SDK client.
type Client struct {
	inner anthropic.Client
}

// NewClient creates a new Anthropic client. Reads ANTHROPIC_API_KEY from the environment.
func NewClient() *Client {
	return &Client{inner: anthropic.NewClient()}
}

// BuildRequests constructs the inline request slice for CreateBatch.
func BuildRequests(images []ImageInput, model, prompt string) []anthropic.BetaMessageBatchNewParamsRequest {
	reqs := make([]anthropic.BetaMessageBatchNewParamsRequest, 0, len(images))
	for _, img := range images {
		reqs = append(reqs, anthropic.BetaMessageBatchNewParamsRequest{
			CustomID: img.CustomID,
			Params: anthropic.BetaMessageBatchNewParamsRequestParams{
				Model:     anthropic.Model(model),
				MaxTokens: 4096,
				Messages: []anthropic.BetaMessageParam{
					anthropic.NewBetaUserMessage(
						anthropic.NewBetaTextBlock(prompt),
						anthropic.NewBetaImageBlock(anthropic.BetaURLImageSourceParam{
							URL: img.ImageURL,
						}),
					),
				},
			},
		})
	}
	return reqs
}

// CreateBatch submits a batch to Anthropic.
func (c *Client) CreateBatch(ctx context.Context, requests []anthropic.BetaMessageBatchNewParamsRequest) (*anthropic.BetaMessageBatch, error) {
	batch, err := c.inner.Beta.Messages.Batches.New(ctx, anthropic.BetaMessageBatchNewParams{
		Requests: requests,
	})
	if err != nil {
		return nil, fmt.Errorf("creating batch: %w", err)
	}
	return batch, nil
}

// GetBatch polls the current status of a batch.
func (c *Client) GetBatch(ctx context.Context, batchID string) (*anthropic.BetaMessageBatch, error) {
	batch, err := c.inner.Beta.Messages.Batches.Get(ctx, batchID, anthropic.BetaMessageBatchGetParams{})
	if err != nil {
		return nil, fmt.Errorf("getting batch: %w", err)
	}
	return batch, nil
}

// DownloadResults streams all results for a batch in "ended" state.
func (c *Client) DownloadResults(ctx context.Context, batchID string) ([]BatchResult, error) {
	stream := c.inner.Beta.Messages.Batches.ResultsStreaming(ctx, batchID, anthropic.BetaMessageBatchResultsParams{})
	var results []BatchResult
	for stream.Next() {
		results = append(results, ExtractResult(stream.Current()))
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("streaming batch results: %w", err)
	}
	return results, nil
}

// ExtractResult parses a single batch result item into a BatchResult.
// Exported for testing.
func ExtractResult(item anthropic.BetaMessageBatchIndividualResponse) BatchResult {
	result := BatchResult{CustomID: item.CustomID}
	raw, _ := json.Marshal(item)
	result.RawJSON = raw

	switch item.Result.Type {
	case "succeeded":
		for _, block := range item.Result.Message.Content {
			if block.Type == "text" {
				result.Content = block.Text
				return result
			}
		}
		result.Err = fmt.Errorf("no text block in succeeded result")
	case "errored":
		result.Err = fmt.Errorf("request errored")
	case "expired":
		result.Err = fmt.Errorf("request expired")
	default:
		result.Err = fmt.Errorf("unknown result type: %s", item.Result.Type)
	}
	return result
}
