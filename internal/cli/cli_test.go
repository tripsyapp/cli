package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCommandsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"commands", "--json", "--config-dir", t.TempDir()}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}

	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["ok"] != true {
		t.Fatalf("ok = %v, want true", envelope["ok"])
	}
	if _, ok := envelope["data"].([]any); !ok {
		t.Fatalf("data = %T, want []any", envelope["data"])
	}
}

func TestRequestRequiresMethodAndPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"request", "GET", "--config-dir", t.TempDir()}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("request requires METHOD and PATH")) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestFormatFullObjectShowsDocumentedAndExtraFields(t *testing.T) {
	got := formatFullObject("Activity", map[string]any{
		"id":            202,
		"name":          "Colosseum Tour",
		"activity_type": "sightseeing",
		"notes":         "Bring tickets",
		"address":       "Piazza del Colosseo",
		"latitude":      41.8902,
		"longitude":     12.4922,
		"period":        nil,
		"documents":     []any{},
		"custom_field":  "kept",
	}, activityDetailFields...)

	for _, want := range []string{
		"Activity",
		"id: 202",
		"name: Colosseum Tour",
		"activity_type: sightseeing",
		"notes: Bring tickets",
		"address: Piazza del Colosseo",
		"latitude: 41.8902",
		"longitude: 12.4922",
		"period: null",
		"documents:",
		"custom_field: kept",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatFullObject missing %q in:\n%s", want, got)
		}
	}
}
