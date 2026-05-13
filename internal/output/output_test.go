package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderJSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	err := Render(&out, Options{JSON: true}, Result{
		Data:    map[string]any{"id": 42},
		Summary: "created",
		Breadcrumbs: []Breadcrumb{
			{Action: "show", Cmd: "tripsy trips show 42"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var envelope Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK {
		t.Fatal("OK = false, want true")
	}
	if envelope.Summary != "created" {
		t.Fatalf("Summary = %q, want created", envelope.Summary)
	}
	if len(envelope.Breadcrumbs) != 1 || envelope.Breadcrumbs[0].Action != "show" {
		t.Fatalf("Breadcrumbs = %#v, want show breadcrumb", envelope.Breadcrumbs)
	}
}

func TestRenderQuietWritesRawData(t *testing.T) {
	var out bytes.Buffer
	if err := Render(&out, Options{Quiet: true}, Result{Data: map[string]any{"ok": true}, Summary: "ignored"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), `"summary"`) {
		t.Fatalf("quiet output should not include envelope summary: %s", out.String())
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("quiet output = %s, want raw data", out.String())
	}
}

func TestRenderHumanAddsTrailingNewline(t *testing.T) {
	var out bytes.Buffer
	if err := Render(&out, Options{IsTerminal: true}, Result{Human: "Hello"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Hello\n" {
		t.Fatalf("human output = %q, want trailing newline", out.String())
	}
}

func TestRenderErrorJSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	if err := RenderError(&out, Options{JSON: true}, "bad token"); err != nil {
		t.Fatal(err)
	}

	var envelope ErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK {
		t.Fatal("OK = true, want false")
	}
	if envelope.Error != "bad token" {
		t.Fatalf("Error = %q, want bad token", envelope.Error)
	}
}
