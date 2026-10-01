package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tripsyapp/cli/internal/api"
	"github.com/tripsyapp/cli/internal/config"
)

func TestDocumentReadAndMutationRouting(t *testing.T) {
	for _, parent := range []string{"trip", "activity", "hosting", "transportation"} {
		t.Run(parent, func(t *testing.T) {
			parentFields := map[string]any{"trip_id": "12", "parent_type": parent}
			suffix := ""
			if parent != "trip" {
				parentFields["parent_id"] = "34"
				suffix = "/" + parent + "/34"
			}
			calls := 0
			session, cleanup := connectTestSessionOptions(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if r.Header.Get("Authorization") != "Token test-token" {
					t.Error("missing request auth")
				}
				if r.Method == "GET" {
					switch r.URL.Path {
					case "/v2/trip/12" + suffix + "/documents":
						if r.URL.Query().Get("page") == "2" {
							_, _ = w.Write([]byte(`{"results":[{"id":57}],"next":null}`))
						} else {
							_, _ = w.Write([]byte(`{"results":[{"id":56,"activity":{"id":34}}],"next":"/v2/trip/12` + suffix + `/documents?page=2"}`))
						}
					case "/v2/trip/12" + suffix + "/documents/56":
						_, _ = w.Write([]byte(`{"id":56,"description":"Original","file_type":"application/pdf"}`))
					case "/v1/documents/56/get":
						_, _ = w.Write([]byte(`{"download_url":"https://tripsy.s3.amazonaws.com/signed","expires_at":"2026-10-01T20:00:00Z"}`))
					default:
						t.Errorf("unexpected path %s", r.URL)
					}
				} else if r.Method == "POST" {
					if r.URL.Path != "/v1/trip/12"+suffix+"/documents" {
						t.Errorf("attach path %s", r.URL)
					}
					var payload map[string]any
					_ = json.NewDecoder(r.Body).Decode(&payload)
					if payload["url"] != "https://example.com/ticket" || payload["file_type"] != "url" {
						t.Errorf("invalid attachment payload %v", payload)
					}
					_, _ = w.Write([]byte(`{"id":56}`))
				} else if r.Method == "DELETE" {
					if r.URL.Path != "/v1/trip/12"+suffix+"/documents/56" {
						t.Errorf("delete path %s", r.URL)
					}
				} else {
					t.Errorf("unexpected method %s", r.Method)
				}
			}), Options{DisableRawRequest: true})
			defer cleanup()
			list := callTool(t, session, "tripsy_documents_list", parentFields)
			if list.IsError {
				t.Fatal(toolText(list))
			}
			if len(structuredMap(t, list)["data"].(map[string]any)["results"].([]any)) != 2 {
				t.Fatal(toolText(list))
			}
			parentFields["id"] = "56"
			res := callTool(t, session, "tripsy_documents_show", parentFields)
			if res.IsError || !strings.Contains(toolText(res), "Original") {
				t.Fatal(toolText(res))
			}
			res = callTool(t, session, "tripsy_documents_get", map[string]any{"id": "56"})
			if res.IsError || !strings.Contains(toolText(res), "expires_at") {
				t.Fatal(toolText(res))
			}
			delete(parentFields, "id")
			parentFields["url"] = "https://example.com/ticket"
			res = callTool(t, session, "tripsy_documents_attach", parentFields)
			if res.IsError {
				t.Fatal(toolText(res))
			}
			delete(parentFields, "url")
			parentFields["id"] = "56"
			res = callTool(t, session, "tripsy_documents_delete", parentFields)
			if res.IsError {
				t.Fatal(toolText(res))
			}
			if calls != 6 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestDocumentUpdatePreservesFieldsAndMovesExclusively(t *testing.T) {
	calls := 0
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "PATCH" || r.URL.Path != "/v1/documents/56" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if len(payload) != 3 || payload["title"] != "" || payload["description"] != "Updated" || payload["hosting_id"] != "34" {
			t.Errorf("wrong payload %v", payload)
		}
	}))
	defer cleanup()
	res := callTool(t, session, "tripsy_documents_update", map[string]any{"id": "56", "title": "", "description": "Updated", "hosting_id": "34"})
	if res.IsError {
		t.Fatal(toolText(res))
	}
	for _, input := range []map[string]any{{"id": "56"}, {"id": "56", "trip_id": "12", "activity_id": "34"}, {"id": "56", "trip_id": " "}} {
		if res := callTool(t, session, "tripsy_documents_update", input); !res.IsError {
			t.Fatal("invalid update accepted")
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestDocumentUploadPrepareAndFileFinalization(t *testing.T) {
	calls := 0
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if r.URL.Path == "/v1/storage/uploads" {
			if payload["purpose"] != "document" || payload["visibility"] != "private" || payload["parent_type"] != "activity" || payload["parent_id"] != "34" || payload["content_length"] != float64(1024) {
				t.Errorf("wrong upload payload %v", payload)
			}
			_, _ = w.Write([]byte(`{"object_key":"documents/private/a.pdf","upload_token":"signed-receipt","visibility":"private"}`))
		} else if r.URL.Path == "/v1/trip/12/activity/34/documents" {
			if payload["upload_token"] != "signed-receipt" || payload["file_type"] != "application/pdf" {
				t.Errorf("wrong finalize payload %v", payload)
			}
			_, _ = w.Write([]byte(`{"id":56}`))
		} else {
			t.Errorf("unexpected path %s", r.URL)
		}
	}))
	defer cleanup()
	parent := map[string]any{"trip_id": "12", "parent_type": "activity", "parent_id": "34", "filename": "ticket.pdf", "content_type": "application/pdf", "content_length": 1024}
	res := callTool(t, session, "tripsy_documents_upload_prepare", parent)
	if res.IsError {
		t.Fatal(toolText(res))
	}
	res = callTool(t, session, "tripsy_documents_attach", map[string]any{"trip_id": "12", "parent_type": "activity", "parent_id": "34", "url": "documents/private/a.pdf", "file_type": "application/pdf", "upload_token": "signed-receipt"})
	if res.IsError {
		t.Fatal(toolText(res))
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestDocumentInvalidUploadsNeverReachAPI(t *testing.T) {
	calls := 0
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(403) }))
	defer cleanup()
	for _, input := range []map[string]any{
		{"trip_id": "1", "filename": "/etc/passwd", "content_type": "text/plain", "content_length": 10},
		{"trip_id": "1", "filename": "a.pdf", "content_type": "application/pdf", "content_length": 26214401},
		{"trip_id": "1", "filename": "a.pdf", "content_type": "application/pdf", "content_length": 0},
	} {
		if res := callTool(t, session, "tripsy_documents_upload_prepare", input); !res.IsError {
			t.Fatal("invalid upload accepted")
		}
	}
	for _, input := range []map[string]any{
		{"trip_id": "1", "url": "file:///etc/passwd"},
		{"trip_id": "1", "url": "foreign.pdf", "file_type": "application/pdf"},
		{"trip_id": "1", "url": "documents/private/a.pdf", "file_type": "application/pdf"},
	} {
		if res := callTool(t, session, "tripsy_documents_attach", input); !res.IsError {
			t.Fatal("invalid attachment accepted")
		}
	}
	for _, content := range []string{"invalid base64", "", strings.Repeat("A", 12*1024*1024)} {
		if res := callTool(t, session, "tripsy_documents_upload", map[string]any{"trip_id": "1", "filename": "a.pdf", "content_type": "application/pdf", "content_base64": content}); !res.IsError {
			t.Fatal("invalid inline upload accepted")
		}
	}
	if calls != 0 {
		t.Fatal("invalid input reached API")
	}
	res := callTool(t, session, "tripsy_documents_get", map[string]any{"id": "56"})
	if !res.IsError || !strings.Contains(toolText(res), "403") {
		t.Fatal(toolText(res))
	}
}

type documentRoundTripper func(*http.Request) (*http.Response, error)

func (f documentRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInlineDocumentUploadUsesS3WithoutCredentialsAndOnlyAttachesAfterSuccess(t *testing.T) {
	for _, statuses := range [][2]int{{200, 200}, {403, 200}, {200, 403}} {
		uploadStatus, attachStatus := statuses[0], statuses[1]
		t.Run(fmt.Sprint(statuses), func(t *testing.T) {
			calls := []string{}
			apiClient := api.NewClient("https://api.test", "test-token")
			apiClient.HTTPClient = &http.Client{Transport: documentRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls = append(calls, r.Method+" "+r.URL.Host+r.URL.Path)
				code, body := 200, `{}`
				if r.URL.Host == "private-docs.s3.amazonaws.com" {
					if r.Header.Get("Authorization") != "" {
						t.Error("Tripsy credentials sent to S3")
					}
					data, _ := io.ReadAll(r.Body)
					if string(data) != "ticket" || r.ContentLength != 6 || r.Method != "PUT" {
						t.Errorf("incorrect uploaded bytes: %q length %d", data, r.ContentLength)
					}
					code = uploadStatus
				} else {
					if r.Header.Get("Authorization") != "Token test-token" {
						t.Error("API auth missing")
					}
					if r.URL.Path == "/v1/storage/uploads" {
						body = `{"upload_url":"https://private-docs.s3.amazonaws.com/documents/private/a.pdf?X-Amz-Signature=test","bucket":"private-docs","visibility":"private","method":"PUT","object_key":"documents/private/a.pdf","upload_token":"receipt","headers":{"Content-Type":"application/pdf","Content-Length":"6"}}`
					} else if r.URL.Path == "/v1/trip/12/documents" {
						var payload map[string]any
						_ = json.NewDecoder(r.Body).Decode(&payload)
						if payload["upload_token"] != "receipt" || payload["url"] != "documents/private/a.pdf" || payload["title"] != "ticket.pdf" {
							t.Errorf("bad finalization %v", payload)
						}
						code, body = attachStatus, `{"id":56}`
					} else {
						t.Errorf("unexpected API path %s", r.URL)
					}
				}
				return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})}
			server := NewWithClientOptions(apiClient, config.NewStore(t.TempDir()), Options{DisableRawRequest: true})
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			ss, err := server.Connect(testContext(t), serverTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()
			client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
			session, err := client.Connect(testContext(t), clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			res := callTool(t, session, "tripsy_documents_upload", map[string]any{"trip_id": "12", "filename": "ticket.pdf", "content_type": "application/pdf", "content_base64": "dGlja2V0"})
			if res.IsError != (uploadStatus != 200 || attachStatus != 200) {
				t.Fatal(toolText(res))
			}
			if uploadStatus == 200 && attachStatus != 200 {
				var envelope struct {
					Data struct {
						Uploaded       bool                `json:"uploaded"`
						RetryTool      string              `json:"retry_tool"`
						RetryArguments documentAttachInput `json:"retry_arguments"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(toolText(res)), &envelope); err != nil {
					t.Fatal(err)
				}
				recovery := envelope.Data
				if !recovery.Uploaded || recovery.RetryTool != "tripsy_documents_attach" || recovery.RetryArguments.UploadToken != "receipt" || recovery.RetryArguments.URL != "documents/private/a.pdf" || recovery.RetryArguments.TripID != "12" || recovery.RetryArguments.FileType != "application/pdf" {
					t.Fatalf("missing recovery details: %+v", recovery)
				}
			}
			want := []string{"POST api.test/v1/storage/uploads", "PUT private-docs.s3.amazonaws.com/documents/private/a.pdf"}
			if uploadStatus == 200 {
				want = append(want, "POST api.test/v1/trip/12/documents")
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want %v", calls, want)
			}
		})
	}
}

func TestDocumentUploadRejectsUntrustedDestinationsAndHeaders(t *testing.T) {
	client := api.NewClient("https://api.test", "secret-token")
	called := false
	client.HTTPClient = &http.Client{Transport: documentRoundTripper(func(r *http.Request) (*http.Response, error) {
		called = true
		return nil, fmt.Errorf("unexpected request")
	})}
	for _, target := range []string{"http://private-docs.s3.amazonaws.com/a?X-Amz-Signature=test", "https://localhost/a?X-Amz-Signature=test", "https://private-docs.s3.amazonaws.com.attacker.test/a?X-Amz-Signature=test", "https://other.s3.amazonaws.com/a?X-Amz-Signature=test", "https://private-docs.s3.amazonaws.com/a"} {
		if err := client.UploadDocumentBytes(context.Background(), target, "private-docs", nil, []byte("ticket")); err == nil {
			t.Fatalf("accepted %s", target)
		}
	}
	target := "https://private-docs.s3.amazonaws.com/a?X-Amz-Signature=test"
	for _, headers := range []map[string]string{{"Authorization": "secret"}, {"x-amz-acl": "public-read"}, {"Content-Length": "1"}} {
		if err := client.UploadDocumentBytes(context.Background(), target, "private-docs", headers, []byte("ticket")); err == nil {
			t.Fatalf("accepted headers %v", headers)
		}
	}
	if called {
		t.Fatal("unsafe upload reached transport")
	}
}
