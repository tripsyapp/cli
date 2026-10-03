package mcpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// These expectations describe behavior, independently of the registration helpers.
var submissionAnnotationCases = []struct {
	name                             string
	readOnly, destructive, openWorld bool
}{
	{"tripsy_activities_create", false, false, false},
	{"tripsy_activities_update", false, true, false},
	{"tripsy_categories_create", false, false, false},
	{"tripsy_categories_update", false, true, false},
	{"tripsy_collaborators_invite", false, true, true},
	{"tripsy_collaborators_update", false, true, false},
	{"tripsy_documents_attach", false, false, true},
	{"tripsy_documents_update", false, true, true},
	{"tripsy_documents_upload", false, false, true},
	{"tripsy_expenses_create", false, false, false},
	{"tripsy_expenses_update", false, true, false},
	{"tripsy_hostings_create", false, false, false},
	{"tripsy_hostings_update", false, true, false},
	{"tripsy_inbox_update", false, true, true},
	{"tripsy_me_update", false, true, false},
	{"tripsy_transportations_create", false, false, false},
	{"tripsy_transportations_update", false, true, false},
	{"tripsy_trips_create", false, true, true},
	{"tripsy_trips_update", false, true, false},
	{"tripsy_documents_get", true, false, false},
	{"tripsy_documents_upload_prepare", false, false, false},
	{"tripsy_trips_list", true, false, false},
	{"tripsy_trips_delete", false, true, false},
	{"tripsy_raw_request", false, true, false},
}

func TestSubmissionToolAnnotations(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.NotFoundHandler())
	defer cleanup()
	listed, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range submissionAnnotationCases {
		t.Run(tt.name, func(t *testing.T) {
			tool := findTool(listed.Tools, tt.name)
			if tool == nil || tool.Annotations == nil {
				t.Fatal("missing tool or annotations")
			}
			a := tool.Annotations
			if a.ReadOnlyHint != tt.readOnly || a.DestructiveHint == nil || *a.DestructiveHint != tt.destructive || a.OpenWorldHint == nil || *a.OpenWorldHint != tt.openWorld {
				t.Fatalf("incorrect annotations: %#v", a)
			}
			if tt.openWorld && (!strings.Contains(tool.Description, "Open-world:") || strings.Contains(tool.Description, "Closed-world:")) {
				t.Fatalf("external effects must be explained without a closed-world claim: %s", tool.Description)
			}
			if tt.destructive && !strings.Contains(tool.Description, "Destructive:") {
				t.Fatalf("destructive effects must be explained: %s", tool.Description)
			}
			if strings.HasSuffix(tt.name, "_update") && !a.IdempotentHint {
				t.Fatal("updates should retain their idempotent hint")
			}
		})
	}
}

func TestNormalizeToolAnnotationsPayloadMatchesSubmissionBehavior(t *testing.T) {
	for _, format := range []string{"json", "sse"} {
		t.Run(format, func(t *testing.T) {
			var tools []map[string]any
			for _, tt := range submissionAnnotationCases {
				tools = append(tools, map[string]any{"name": tt.name})
			}
			payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"tools": tools}})
			if err != nil {
				t.Fatal(err)
			}
			if format == "sse" {
				payload = append(append([]byte("event: message\ndata: "), payload...), '\n', '\n')
			} else {
				payload = append(payload, '\n')
			}
			normalized := NormalizeToolAnnotationsPayload(payload)
			if !bytes.HasSuffix(normalized, []byte("\n")) {
				t.Fatal("message delimiter was lost")
			}
			if format == "sse" {
				if !bytes.HasSuffix(normalized, []byte("\n\n")) {
					t.Fatal("SSE event delimiter was lost")
				}
				normalized = normalizedJSONFromSSEDataLine(t, normalized)
			}
			var out struct {
				Result struct {
					Tools []struct {
						Annotations map[string]bool `json:"annotations"`
					} `json:"tools"`
				} `json:"result"`
			}
			if err := json.Unmarshal(normalized, &out); err != nil {
				t.Fatal(err)
			}
			if len(out.Result.Tools) != len(submissionAnnotationCases) {
				t.Fatal("tools changed during normalization")
			}
			for i, tt := range submissionAnnotationCases {
				for key, want := range map[string]bool{"readOnlyHint": tt.readOnly, "destructiveHint": tt.destructive, "openWorldHint": tt.openWorld} {
					got, ok := out.Result.Tools[i].Annotations[key]
					if !ok || got != want {
						t.Errorf("%s %s = %v (present: %v), want %v", tt.name, key, got, ok, want)
					}
				}
			}
			if !bytes.Equal(NormalizeToolAnnotationsPayload(normalized), normalized) {
				t.Fatal("normalizing explicit annotations should preserve them")
			}
		})
	}
}

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
