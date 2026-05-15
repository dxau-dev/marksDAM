package anthropic_test

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	markdam "github.com/dxau-dev/marksDAM/anthropic"
)

func TestBuildRequests_Count(t *testing.T) {
	images := []markdam.ImageInput{
		{CustomID: "img-1", ImageURL: "https://example.com/a.jpg"},
		{CustomID: "img-2", ImageURL: "https://example.com/b.jpg"},
	}
	reqs := markdam.BuildRequests(images, "claude-sonnet-4-5", "describe it")
	if len(reqs) != 2 {
		t.Fatalf("BuildRequests() returned %d requests, want 2", len(reqs))
	}
}

func TestBuildRequests_CustomIDs(t *testing.T) {
	images := []markdam.ImageInput{
		{CustomID: "img-42", ImageURL: "https://example.com/cat.jpg"},
	}
	reqs := markdam.BuildRequests(images, "claude-sonnet-4-5", "describe it")
	// CustomID is a plain string field on BetaMessageBatchNewParamsRequest.
	if reqs[0].CustomID != "img-42" {
		t.Errorf("CustomID = %q, want %q", reqs[0].CustomID, "img-42")
	}
}

func TestBuildRequests_Model(t *testing.T) {
	images := []markdam.ImageInput{
		{CustomID: "img-1", ImageURL: "https://example.com/a.jpg"},
	}
	reqs := markdam.BuildRequests(images, "claude-opus-4-7", "describe it")
	// Model is a plain Model field on BetaMessageBatchNewParamsRequestParams.
	model := string(reqs[0].Params.Model)
	if model != "claude-opus-4-7" {
		t.Errorf("Model = %q, want %q", model, "claude-opus-4-7")
	}
}

func TestExtractResult_Succeeded(t *testing.T) {
	item := anthropic.BetaMessageBatchIndividualResponse{
		CustomID: "img-1",
		Result: anthropic.BetaMessageBatchResultUnion{
			Type: "succeeded",
			Message: anthropic.BetaMessage{
				Content: []anthropic.BetaContentBlockUnion{
					{Type: "text", Text: "A cat on a mat"},
				},
			},
		},
	}
	result := markdam.ExtractResult(item)
	if result.CustomID != "img-1" {
		t.Errorf("CustomID = %q, want %q", result.CustomID, "img-1")
	}
	if result.Content != "A cat on a mat" {
		t.Errorf("Content = %q, want %q", result.Content, "A cat on a mat")
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil", result.Err)
	}
}

func TestExtractResult_Errored(t *testing.T) {
	item := anthropic.BetaMessageBatchIndividualResponse{
		CustomID: "img-2",
		Result: anthropic.BetaMessageBatchResultUnion{
			Type: "errored",
		},
	}
	result := markdam.ExtractResult(item)
	if result.Err == nil {
		t.Error("Err = nil, want non-nil error for errored result")
	}
}

func TestExtractResult_Expired(t *testing.T) {
	item := anthropic.BetaMessageBatchIndividualResponse{
		CustomID: "img-3",
		Result: anthropic.BetaMessageBatchResultUnion{
			Type: "expired",
		},
	}
	result := markdam.ExtractResult(item)
	if result.Err == nil {
		t.Error("Err = nil, want non-nil error for expired result")
	}
}

func TestExtractResult_NoTextBlock(t *testing.T) {
	item := anthropic.BetaMessageBatchIndividualResponse{
		CustomID: "img-4",
		Result: anthropic.BetaMessageBatchResultUnion{
			Type:    "succeeded",
			Message: anthropic.BetaMessage{},
		},
	}
	result := markdam.ExtractResult(item)
	if result.Err == nil {
		t.Error("Err = nil, want non-nil error when no text block found")
	}
}
