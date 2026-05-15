package openai

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestBuildBatchRequest(t *testing.T) {
	req := BuildBatchRequest("id-1", "https://example.com/img.jpg", "gpt-4.1", "describe it", "high")

	if req.CustomID != "id-1" {
		t.Errorf("CustomID = %q, want %q", req.CustomID, "id-1")
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want %q", req.Method, "POST")
	}
	if req.URL != "/v1/chat/completions" {
		t.Errorf("URL = %q, want %q", req.URL, "/v1/chat/completions")
	}
	if req.Body.Model != "gpt-4.1" {
		t.Errorf("Body.Model = %q, want %q", req.Body.Model, "gpt-4.1")
	}
	if req.Body.MaxTokens != 4096 {
		t.Errorf("Body.MaxTokens = %d, want 4096", req.Body.MaxTokens)
	}
	if len(req.Body.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(req.Body.Messages))
	}
}

func TestBuildJSONL(t *testing.T) {
	reqs := []BatchRequest{
		BuildBatchRequest("id-1", "https://example.com/a.jpg", "gpt-4.1", "test", "high"),
		BuildBatchRequest("id-2", "https://example.com/b.jpg", "gpt-4.1", "test", "low"),
	}
	data, err := BuildJSONL(reqs)
	if err != nil {
		t.Fatalf("BuildJSONL: %v", err)
	}

	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d", len(lines))
	}

	var decoded BatchRequest
	if err := json.Unmarshal(lines[0], &decoded); err != nil {
		t.Errorf("line 0 is not valid JSON: %v", err)
	}
	if decoded.CustomID != "id-1" {
		t.Errorf("decoded CustomID = %q, want %q", decoded.CustomID, "id-1")
	}
}

func TestParseAndExtractContent(t *testing.T) {
	successBody := `{"id":"chatcmpl-1","choices":[{"index":0,"message":{"role":"assistant","content":"A cat sitting on a mat"}}]}`

	tests := []struct {
		name    string
		jsonl   string
		want    string
		wantErr bool
	}{
		{
			name:    "successful response",
			jsonl:   `{"id":"r1","custom_id":"img-1","response":{"status_code":200,"body":` + successBody + `}}`,
			want:    "A cat sitting on a mat",
			wantErr: false,
		},
		{
			name:    "error response",
			jsonl:   `{"id":"r1","custom_id":"img-1","error":{"code":"invalid_request","message":"bad request"}}`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty choices",
			jsonl:   `{"id":"r1","custom_id":"img-1","response":{"status_code":200,"body":{"id":"chatcmpl-1","choices":[]}}}`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "nil response body",
			jsonl:   `{"id":"r1","custom_id":"img-1"}`,
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses, err := ParseBatchOutput([]byte(tt.jsonl))
			if err != nil {
				t.Fatalf("ParseBatchOutput: %v", err)
			}
			if len(responses) != 1 {
				t.Fatalf("expected 1 response, got %d", len(responses))
			}
			content, err := ExtractContent(responses[0])
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractContent() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if content != tt.want {
				t.Errorf("ExtractContent() = %q, want %q", content, tt.want)
			}
		})
	}
}

func TestParseBatchOutput_Malformed(t *testing.T) {
	_, err := ParseBatchOutput([]byte(`{not valid json`))
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}
