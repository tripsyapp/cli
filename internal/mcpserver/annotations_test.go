package mcpserver

import (
	"encoding/json"
	"testing"
)

func TestNormalizeToolAnnotationsPayloadAddsExplicitFalseHints(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"tripsy_trips_create","annotations":{"destructiveHint":false,"openWorldHint":false}},{"name":"tripsy_trips_list","annotations":{"readOnlyHint":true,"openWorldHint":false}}]}}`)

	normalized := NormalizeToolAnnotationsPayload(payload)
	var out struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				Annotations map[string]any `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(normalized, &out); err != nil {
		t.Fatalf("unmarshal normalized payload: %v\n%s", err, normalized)
	}

	for _, tool := range out.Result.Tools {
		for _, key := range []string{"readOnlyHint", "openWorldHint", "destructiveHint"} {
			if _, ok := tool.Annotations[key]; !ok {
				t.Fatalf("tool %s is missing %s in %s", tool.Name, key, normalized)
			}
		}
	}
	if out.Result.Tools[0].Annotations["readOnlyHint"] != false {
		t.Fatalf("create tool readOnlyHint = %v, want false", out.Result.Tools[0].Annotations["readOnlyHint"])
	}
	if out.Result.Tools[1].Annotations["destructiveHint"] != false {
		t.Fatalf("list tool destructiveHint = %v, want false", out.Result.Tools[1].Annotations["destructiveHint"])
	}
}

func TestNormalizeToolAnnotationsPayloadHandlesSSEDataLines(t *testing.T) {
	payload := []byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"tripsy_trips_delete\",\"annotations\":{\"destructiveHint\":true,\"openWorldHint\":false}}]}}\n\n")

	normalized := NormalizeToolAnnotationsPayload(payload)
	if string(normalized) == string(payload) {
		t.Fatal("expected SSE payload to be normalized")
	}
	if !json.Valid(normalizedJSONFromSSEDataLine(t, normalized)) {
		t.Fatalf("normalized SSE data line should contain valid JSON: %s", normalized)
	}
}

func normalizedJSONFromSSEDataLine(t *testing.T, payload []byte) []byte {
	t.Helper()
	const prefix = "data: "
	lines := string(payload)
	start := len("event: message\n")
	if len(lines) <= start+len(prefix) || lines[start:start+len(prefix)] != prefix {
		t.Fatalf("payload is missing data line: %s", payload)
	}
	end := start + len(prefix)
	for end < len(lines) && lines[end] != '\n' {
		end++
	}
	return []byte(lines[start+len(prefix) : end])
}
