package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tripsyapp/cli/internal/api"
	"github.com/tripsyapp/cli/internal/config"
)

func TestListToolsIncludesCoreTripsySurface(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.NotFoundHandler())
	defer cleanup()

	res, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}

	for _, name := range []string{
		"tripsy_status",
		"tripsy_itinerary_guidance",
		"tripsy_trips_create",
		"tripsy_activities_create",
		"tripsy_hostings_create",
		"tripsy_transportations_create",
		"tripsy_categories_create",
		"tripsy_collaborators_list",
		"tripsy_trips_list",
		"tripsy_trips_following_list",
		"tripsy_raw_request",
	} {
		if findTool(res.Tools, name) == nil {
			t.Fatalf("tool %q was not registered", name)
		}
	}

	for _, name := range []string{
		"tripsy_emails_list",
		"tripsy_inbox_list",
		"tripsy_documents_attach",
		"tripsy_documents_upload",
		"tripsy_uploads_create",
	} {
		if findTool(res.Tools, name) != nil {
			t.Fatalf("tool %q should not be registered yet", name)
		}
	}

	allowedToolName := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	for _, tool := range res.Tools {
		if !allowedToolName.MatchString(tool.Name) {
			t.Fatalf("tool name %q contains characters rejected by common MCP clients", tool.Name)
		}
		if tool.Annotations == nil {
			t.Fatalf("tool %q is missing annotations", tool.Name)
		}
		if tool.Annotations.OpenWorldHint == nil {
			t.Fatalf("tool %q is missing openWorldHint", tool.Name)
		}
		if tool.Annotations.DestructiveHint == nil {
			t.Fatalf("tool %q is missing destructiveHint", tool.Name)
		}
		outputSchema := outputSchemaString(t, tool)
		if !strings.Contains(outputSchema, "summary") || !strings.Contains(outputSchema, "status_code") || !strings.Contains(outputSchema, "data") {
			t.Fatalf("tool %q output schema should describe the standard response envelope: %s", tool.Name, outputSchema)
		}
		if tool.Annotations != nil && tool.Annotations.OpenWorldHint != nil && !*tool.Annotations.OpenWorldHint && !strings.Contains(tool.Description, "Closed-world:") {
			t.Fatalf("tool %q description should explain closed-world safety hint: %q", tool.Name, tool.Description)
		}
		if tool.Annotations != nil && tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint && !strings.Contains(tool.Description, "Destructive:") {
			t.Fatalf("tool %q description should explain destructive safety hint: %q", tool.Name, tool.Description)
		}
	}

	activitiesList := findTool(res.Tools, "tripsy_activities_list")
	if activitiesList == nil {
		t.Fatal("tripsy_activities_list was not registered")
	}
	if activitiesList.Title != "List Activities" {
		t.Fatalf("activities list title = %q, want List Activities", activitiesList.Title)
	}
	if activitiesList.Annotations == nil || !activitiesList.Annotations.ReadOnlyHint {
		t.Fatal("activities list should be marked read-only")
	}
	if !strings.Contains(activitiesList.Description, "convert UTC starts_at/ends_at values into the activity timezone") {
		t.Fatalf("activities list description should mention display timezone conversion: %q", activitiesList.Description)
	}
	hostingsList := findTool(res.Tools, "tripsy_hostings_list")
	if !strings.Contains(hostingsList.Description, "convert UTC starts_at/ends_at values into the lodging timezone") {
		t.Fatalf("hostings list description should mention lodging timezone conversion: %q", hostingsList.Description)
	}

	categoriesList := findTool(res.Tools, "tripsy_categories_list")
	if categoriesList == nil {
		t.Fatal("tripsy_categories_list was not registered")
	}
	if !strings.Contains(categoriesList.Description, "Custom category slugs") || !strings.Contains(categoriesList.Description, "activity_type") {
		t.Fatalf("categories list description should mention activity_type usage: %q", categoriesList.Description)
	}
	categoriesCreate := findTool(res.Tools, "tripsy_categories_create")
	if categoriesCreate == nil {
		t.Fatal("tripsy_categories_create was not registered")
	}
	categoriesCreateSchema := toolSchemaString(t, categoriesCreate)
	if !strings.Contains(categoriesCreateSchema, "icon_name") || !strings.Contains(categoriesCreateSchema, "AABBCC") || !strings.Contains(categoriesCreateSchema, "already owns this slug") {
		t.Fatalf("categories create input schema should expose custom category fields: %s", categoriesCreateSchema)
	}

	tripsList := findTool(res.Tools, "tripsy_trips_list")
	if !strings.Contains(tripsList.Description, "user is travelling") || !strings.Contains(tripsList.Description, "is_travelling") || !strings.Contains(tripsList.Description, "has_dates is authoritative") || !strings.Contains(tripsList.Description, "ignore starts_at and ends_at") {
		t.Fatalf("trips list description should mention travelling and has_dates guidance: %q", tripsList.Description)
	}
	tripsFollowingList := findTool(res.Tools, "tripsy_trips_following_list")
	if !strings.Contains(tripsFollowingList.Description, "follows but is not travelling") || !strings.Contains(tripsFollowingList.Description, "is_travelling") || !strings.Contains(tripsFollowingList.Description, "has_dates is authoritative") {
		t.Fatalf("trips following list description should mention following and has_dates guidance: %q", tripsFollowingList.Description)
	}
	tripsListSchema := toolSchemaString(t, tripsList)
	if !strings.Contains(tripsListSchema, "owner") || !strings.Contains(tripsListSchema, "guests") || !strings.Contains(tripsListSchema, "has_dates") || !strings.Contains(tripsListSchema, "is_travelling") || !strings.Contains(tripsListSchema, "undated trips") {
		t.Fatalf("trips list input schema should preserve travelling/date context fields: %s", tripsListSchema)
	}

	tripsCreate := findTool(res.Tools, "tripsy_trips_create")
	if !strings.Contains(tripsCreate.Description, "cover_image_url") || !strings.Contains(tripsCreate.Description, "https://images.unsplash.com/photo-") || !strings.Contains(tripsCreate.Description, "photo-nWdsya5_Yms") || !strings.Contains(tripsCreate.Description, "MCP server validates") || !strings.Contains(tripsCreate.Description, "external URL access") {
		t.Fatalf("trips create description should mention reachable Unsplash cover_image_url guidance: %q", tripsCreate.Description)
	}
	tripsCreateSchema := toolSchemaString(t, tripsCreate)
	if !strings.Contains(tripsCreateSchema, "cover_image_url") || !strings.Contains(tripsCreateSchema, "numeric timestamp") || !strings.Contains(tripsCreateSchema, "photo-nWdsya5_Yms") || !strings.Contains(tripsCreateSchema, "external URL access") || !strings.Contains(tripsCreateSchema, "starts_at") {
		t.Fatalf("trips create input schema should expose typed itinerary fields: %s", tripsCreateSchema)
	}
	tripsDelete := findTool(res.Tools, "tripsy_trips_delete")
	if !strings.Contains(tripsDelete.Description, "may be executed when requested") || !strings.Contains(tripsDelete.Description, "can be undone if necessary") {
		t.Fatalf("trips delete description should mention executable and undo guidance: %q", tripsDelete.Description)
	}

	activitiesCreate := findTool(res.Tools, "tripsy_activities_create")
	if !strings.Contains(activitiesCreate.Description, "one activity per actual stop") || !strings.Contains(activitiesCreate.Description, "Timed values are always UTC") || !strings.Contains(activitiesCreate.Description, "local IANA timezone") || !strings.Contains(activitiesCreate.Description, "convert UTC starts_at/ends_at values into the activity timezone") || !strings.Contains(activitiesCreate.Description, "activity_type") || !strings.Contains(activitiesCreate.Description, "visible custom category slug") || !strings.Contains(activitiesCreate.Description, "only valid on Activity objects") || !strings.Contains(activitiesCreate.Description, "tripsy_categories_list") || !strings.Contains(activitiesCreate.Description, "correct custom activity category name") || !strings.Contains(activitiesCreate.Description, "latitude/longitude") || !strings.Contains(activitiesCreate.Description, "sightseeing") {
		t.Fatalf("activities create description should mention time, category, and coordinates guidance: %q", activitiesCreate.Description)
	}
	activitiesCreateSchema := toolSchemaString(t, activitiesCreate)
	if !strings.Contains(activitiesCreateSchema, "activity_type") || !strings.Contains(activitiesCreateSchema, "visible custom category slug") || !strings.Contains(activitiesCreateSchema, "tripsy_categories_list") || !strings.Contains(activitiesCreateSchema, "only valid on Activity objects") || !strings.Contains(activitiesCreateSchema, "do not invent ad hoc values such as sightseeing") || !strings.Contains(activitiesCreateSchema, "Timed values are always UTC") || !strings.Contains(activitiesCreateSchema, "convert this value to the activity timezone") || !strings.Contains(activitiesCreateSchema, "before displaying the local date/time") || !strings.Contains(activitiesCreateSchema, "latitude") {
		t.Fatalf("activities create input schema should expose typed activity fields: %s", activitiesCreateSchema)
	}

	hostingsCreate := findTool(res.Tools, "tripsy_hostings_create")
	if !strings.Contains(hostingsCreate.Description, "lodging rather than activities") || !strings.Contains(hostingsCreate.Description, "Timed values are always UTC") || !strings.Contains(hostingsCreate.Description, "local IANA timezone") || !strings.Contains(hostingsCreate.Description, "convert UTC starts_at/ends_at values into the lodging timezone") || !strings.Contains(hostingsCreate.Description, "address") || !strings.Contains(hostingsCreate.Description, "latitude") {
		t.Fatalf("hostings create description should mention lodging, time, and coordinates guidance: %q", hostingsCreate.Description)
	}
	hostingsCreateSchema := toolSchemaString(t, hostingsCreate)
	if !strings.Contains(hostingsCreateSchema, "convert this value to the lodging timezone") || !strings.Contains(hostingsCreateSchema, "before displaying the local date/time") || !strings.Contains(hostingsCreateSchema, "UTC starts_at/ends_at values") {
		t.Fatalf("hostings create input schema should expose lodging timezone conversion guidance: %s", hostingsCreateSchema)
	}

	transportationsCreate := findTool(res.Tools, "tripsy_transportations_create")
	if !strings.Contains(transportationsCreate.Description, "Timed values are always UTC") || !strings.Contains(transportationsCreate.Description, "departure_timezone and arrival_timezone") || !strings.Contains(transportationsCreate.Description, "convert departure_at with departure_timezone and arrival_at with arrival_timezone") || !strings.Contains(transportationsCreate.Description, "transfer activities") || !strings.Contains(transportationsCreate.Description, "roadtrip") || !strings.Contains(transportationsCreate.Description, "addresses") || !strings.Contains(transportationsCreate.Description, "airport IATA codes") || !strings.Contains(transportationsCreate.Description, "departure/arrival latitudes and longitudes") || !strings.Contains(transportationsCreate.Description, "omit name") {
		t.Fatalf("transportations create description should mention time, flight IATA/coordinates and transfer roadtrip endpoint guidance: %q", transportationsCreate.Description)
	}
	transportationsCreateSchema := toolSchemaString(t, transportationsCreate)
	if !strings.Contains(transportationsCreateSchema, "transportation_type") || !strings.Contains(transportationsCreateSchema, "For flights, use airplane") || !strings.Contains(transportationsCreateSchema, "airport IATA code") || !strings.Contains(transportationsCreateSchema, "Timed values are always UTC") || !strings.Contains(transportationsCreateSchema, "convert this value to departure_timezone") || !strings.Contains(transportationsCreateSchema, "convert this value to arrival_timezone") || !strings.Contains(transportationsCreateSchema, "Required for flight airports") || !strings.Contains(transportationsCreateSchema, "omit name") || !strings.Contains(transportationsCreateSchema, "For transfer activities, use roadtrip") || !strings.Contains(transportationsCreateSchema, "departure_address") || !strings.Contains(transportationsCreateSchema, "arrival_address") {
		t.Fatalf("transportations create input schema should expose typed transfer fields: %s", transportationsCreateSchema)
	}
	transportationsList := findTool(res.Tools, "tripsy_transportations_list")
	if !strings.Contains(transportationsList.Description, "convert UTC departure_at with departure_timezone and UTC arrival_at with arrival_timezone") {
		t.Fatalf("transportations list description should mention endpoint timezone conversion: %q", transportationsList.Description)
	}
}

func TestTripsListKeepsOwnershipAndDateContextFields(t *testing.T) {
	var called atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path == "/v1/me" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":7}`))
			return
		}
		if r.URL.Path != "/v2/trips/" {
			t.Errorf("path = %s, want /v2/trips/", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("fields"); got != "ends_at,guests,has_dates,id,name,owner,starts_at" {
			t.Errorf("fields = %q, want travelling/date context fields preserved", got)
		}
		if got := query.Get("fields!"); got != "" {
			t.Errorf("fields! = %q, want empty", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":42,"name":"Shared trip","has_dates":false,"starts_at":"2026-06-01","ends_at":"2026-06-03","owner":8,"guests":[{"id":7,"permissions":{"is_travelling":true}}]}]}`))
	})
	session, cleanup := connectTestSession(t, "test-token", handler)
	defer cleanup()

	res := callTool(t, session, "tripsy_trips_list", map[string]any{
		"fields":         []string{"name", "id"},
		"fields_exclude": []string{"owner", "guests", "has_dates"},
	})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}
	if called.Load() != 2 {
		t.Fatalf("handler called %d times, want 2", called.Load())
	}

	structured := structuredMap(t, res)
	summary := fmt.Sprint(structured["summary"])
	if !strings.Contains(summary, "user is travelling") || !strings.Contains(summary, "has_dates is authoritative") {
		t.Fatalf("summary = %q, want travelling and has_dates guidance", summary)
	}
}

func TestTripsFollowingListFiltersNonTravellingTrips(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.RequestURI() {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"id":7}`))
		case "/v2/trips/":
			_, _ = w.Write([]byte(`{"count":2,"next":null,"previous":null,"results":[{"id":42,"name":"Following","owner":8,"guests":[{"id":7,"permissions":{"is_travelling":false}}]},{"id":43,"name":"Travelling","owner":8,"guests":[{"id":7,"permissions":{"is_travelling":true}}]}]}`))
		default:
			t.Fatalf("unexpected request URI: %s", r.URL.RequestURI())
		}
	})
	session, cleanup := connectTestSession(t, "test-token", handler)
	defer cleanup()

	res := callTool(t, session, "tripsy_trips_following_list", map[string]any{})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}

	structured := structuredMap(t, res)
	data := structured["data"].(map[string]any)
	items := data["results"].([]any)
	if len(items) != 1 {
		t.Fatalf("results len = %d, want 1", len(items))
	}
	if got := items[0].(map[string]any)["name"]; got != "Following" {
		t.Fatalf("name = %v, want Following", got)
	}
}

func TestItineraryGuidanceReturnsTripCreationRules(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.NotFoundHandler())
	defer cleanup()

	res := callTool(t, session, "tripsy_itinerary_guidance", map[string]any{})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}

	text := toolText(res)
	for _, want := range []string{
		"cover_image_url",
		"https://images.unsplash.com/photo-",
		"photo-nWdsya5_Yms",
		"unsplash.com/photos",
		"external URL access",
		"one Tripsy item per actual stop",
		"UTC ISO-8601 timestamps",
		"correct local date/time",
		"convert UTC starts_at/ends_at into the activity timezone",
		"convert UTC starts_at/ends_at into the lodging timezone",
		"convert departure_at with departure_timezone and arrival_at with arrival_timezone",
		"airport IATA codes",
		"departure/arrival latitudes and longitudes",
		"omit name unless the user provided one",
		"transportation_type roadtrip",
		"need to be undone",
		"tripsy_categories_list",
		"only valid on Activity objects",
		"Do not treat an unknown activity_type as invalid",
		"Do not invent ad hoc values such as sightseeing",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("itinerary guidance missing %q in: %s", want, text)
		}
	}
}

func TestListToolsCanDisableRawRequest(t *testing.T) {
	session, cleanup := connectTestSessionOptions(t, "test-token", http.NotFoundHandler(), Options{DisableRawRequest: true})
	defer cleanup()

	res, err := session.ListTools(testContext(t), nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	if findTool(res.Tools, "tripsy_raw_request") != nil {
		t.Fatal("tripsy_raw_request should not be registered when disabled")
	}
	if findTool(res.Tools, "tripsy_trips_create") == nil {
		t.Fatal("typed tools should still be registered")
	}
}

func TestNewRequestTokenOnlyIgnoresServerTokenSources(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
	t.Setenv("TRIPSY_TOKEN", "env-token")
	dir := t.TempDir()
	store := config.NewStore(dir)
	if err := store.SaveCredentials(config.Credentials{Token: "stored-token", BaseURL: "https://example.com"}); err != nil {
		t.Fatal(err)
	}

	_, info, err := New(Options{
		ConfigDir:        dir,
		Token:            "option-token",
		RequestTokenOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.HasToken {
		t.Fatal("HasToken = true, want false in request-token-only mode")
	}
	if info.APIBase != "https://example.com" {
		t.Fatalf("APIBase = %q, want non-secret stored base URL", info.APIBase)
	}
}

func TestTripCreateSendsAuthenticatedTripsyRequest(t *testing.T) {
	var called atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/trips" {
			t.Errorf("path = %s, want /v1/trips", r.URL.Path)
		}
		if got := r.URL.Query().Get("fields!"); got != "documents,emails" {
			t.Errorf("fields! = %q, want documents,emails", got)
		}
		if got := r.Header.Get("Authorization"); got != "Token test-token" {
			t.Errorf("Authorization = %q, want Token test-token", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["name"] != "Copenhagen" {
			t.Errorf("body name = %v, want Copenhagen", body["name"])
		}
		if got := fmt.Sprint(body["cover_image_url"]); !strings.HasPrefix(got, "https://images.unsplash.com/photo-") {
			t.Errorf("cover_image_url = %q, want direct Unsplash image URL", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42,"name":"Copenhagen"}`))
	})
	session, cleanup := connectTestSession(t, "test-token", handler)
	defer cleanup()

	res := callTool(t, session, "tripsy_trips_create", map[string]any{
		"data": map[string]any{
			"name":            "Copenhagen",
			"timezone":        "Europe/Copenhagen",
			"cover_image_url": "https://images.unsplash.com/photo-1723596807374-5cfbe183a820?ixid=abc&ixlib=rb-4.1.0",
		},
	})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}

	structured := structuredMap(t, res)
	if got := fmt.Sprint(structured["status_code"]); got != "201" {
		t.Fatalf("status_code = %s, want 201", got)
	}
	if structured["summary"] != "Trip created" {
		t.Fatalf("summary = %v, want Trip created", structured["summary"])
	}
	data := structured["data"].(map[string]any)
	if data["name"] != "Copenhagen" {
		t.Fatalf("data.name = %v, want Copenhagen", data["name"])
	}
}

func TestCategoryCreateSendsAuthenticatedTripsyRequest(t *testing.T) {
	var called atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/categories" {
			t.Errorf("path = %s, want /v1/categories", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Token test-token" {
			t.Errorf("Authorization = %q, want Token test-token", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		for key, want := range map[string]any{
			"name":      "Golf Clubs",
			"slug":      "golf-clubs",
			"icon_name": "golf",
			"color":     "AABBCC",
		} {
			if body[key] != want {
				t.Errorf("body[%s] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":12,"owner":1,"slug":"golf-clubs","icon_name":"golf","name":"Golf Clubs","color":"AABBCC"}`))
	})
	session, cleanup := connectTestSession(t, "test-token", handler)
	defer cleanup()

	res := callTool(t, session, "tripsy_categories_create", map[string]any{
		"name":      "Golf Clubs",
		"slug":      "golf-clubs",
		"icon_name": "golf",
		"color":     "AABBCC",
	})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}

	structured := structuredMap(t, res)
	if got := fmt.Sprint(structured["status_code"]); got != "201" {
		t.Fatalf("status_code = %s, want 201", got)
	}
	if structured["summary"] != "Category created" {
		t.Fatalf("summary = %v, want Category created", structured["summary"])
	}
}

func TestTripCreateRejectsShortUnsplashPhotoID(t *testing.T) {
	var called atomic.Int32
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		t.Errorf("API should not be called with an invalid cover_image_url")
	}))
	defer cleanup()

	res := callTool(t, session, "tripsy_trips_create", map[string]any{
		"name":            "Copenhagen",
		"cover_image_url": "https://images.unsplash.com/photo-nWdsya5_Yms?ixlib=rb-4.1.0",
	})
	if !res.IsError {
		t.Fatalf("expected tool error for invalid cover_image_url, got: %s", toolText(res))
	}
	if !strings.Contains(toolText(res), "numeric Unsplash photo asset path") || !strings.Contains(toolText(res), "photo-nWdsya5_Yms") {
		t.Fatalf("error text = %q, want specific Unsplash URL guidance", toolText(res))
	}
	if called.Load() != 0 {
		t.Fatalf("handler called %d times, want 0", called.Load())
	}
}

func TestTransportationCreateAcceptsTypedTransferFields(t *testing.T) {
	var called atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/trip/42/transportations" {
			t.Errorf("path = %s, want /v1/trip/42/transportations", r.URL.Path)
		}
		if got := r.URL.Query().Get("fields!"); got != "documents,emails" {
			t.Errorf("fields! = %q, want documents,emails", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		for key, want := range map[string]any{
			"name":                  "Transfer to Hotel Eden",
			"transportation_type":   "roadtrip",
			"departure_description": "Rome Fiumicino Airport",
			"departure_address":     "Via dell'Aeroporto di Fiumicino, 00054 Fiumicino RM, Italy",
			"arrival_description":   "Hotel Eden",
			"arrival_address":       "Via Ludovisi 49, 00187 Rome, Italy",
		} {
			if body[key] != want {
				t.Errorf("body[%s] = %v, want %v", key, body[key], want)
			}
		}
		if body["departure_latitude"] != 41.8003 || body["arrival_longitude"] != 12.4882 {
			t.Errorf("coordinates not forwarded: %#v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":303,"name":"Transfer to Hotel Eden"}`))
	})
	session, cleanup := connectTestSession(t, "test-token", handler)
	defer cleanup()

	res := callTool(t, session, "tripsy_transportations_create", map[string]any{
		"trip_id":               "42",
		"name":                  "Transfer to Hotel Eden",
		"transportation_type":   "roadtrip",
		"departure_description": "Rome Fiumicino Airport",
		"departure_address":     "Via dell'Aeroporto di Fiumicino, 00054 Fiumicino RM, Italy",
		"departure_latitude":    41.8003,
		"departure_longitude":   12.2389,
		"arrival_description":   "Hotel Eden",
		"arrival_address":       "Via Ludovisi 49, 00187 Rome, Italy",
		"arrival_latitude":      41.9081,
		"arrival_longitude":     12.4882,
	})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}
}

func TestTypedToolEscapesPathIDs(t *testing.T) {
	var called atomic.Int32
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.URL.EscapedPath() != "/v2/trips/a%2Fb/" {
			t.Errorf("escaped path = %s, want /v2/trips/a%%2Fb/", r.URL.EscapedPath())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"a/b"}`))
	}))
	defer cleanup()

	res := callTool(t, session, "tripsy_trips_show", map[string]any{"id": "a/b"})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}
}

func TestRemoteBearerTokenOverridesStoredMCPToken(t *testing.T) {
	var called atomic.Int32
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if got := r.Header.Get("Authorization"); got != "Token remote-token" {
			t.Errorf("Authorization = %q, want Token remote-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer apiServer.Close()

	service := &service{client: api.NewClient(apiServer.URL, "stored-token"), store: config.NewStore(t.TempDir())}
	req := &mcp.CallToolRequest{
		Extra: &mcp.RequestExtra{
			TokenInfo: &auth.TokenInfo{Extra: map[string]any{tokenInfoTripsyTokenKey: "remote-token"}},
		},
	}
	res, err := service.do(testContext(t), req, "GET", "/v1/trips", nil, nil, "Trips")
	if err != nil {
		t.Fatalf("do() failed: %v", err)
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}
	if got := res.(map[string]any)["summary"]; got != "Trips" {
		t.Fatalf("summary = %v, want Trips", got)
	}
}

func TestRemoteOAuthBearerTokenUsesBearerScheme(t *testing.T) {
	var called atomic.Int32
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Errorf("Authorization = %q, want Bearer oauth-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer apiServer.Close()

	service := &service{client: api.NewClient(apiServer.URL, "stored-token"), store: config.NewStore(t.TempDir())}
	req := &mcp.CallToolRequest{
		Extra: &mcp.RequestExtra{
			TokenInfo: &auth.TokenInfo{Extra: map[string]any{
				tokenInfoTripsyTokenKey: "oauth-token",
				tokenInfoAuthSchemeKey:  "Bearer",
			}},
		},
	}
	if _, err := service.do(testContext(t), req, "GET", "/v1/trips", nil, nil, "Trips"); err != nil {
		t.Fatalf("do() failed: %v", err)
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}
}

func TestBearerTokenVerifierValidatesTripsyToken(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Token valid-token" {
			t.Errorf("Authorization = %q, want Token valid-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"email":"user@example.com"}`))
	}))
	defer apiServer.Close()

	info, err := BearerTokenVerifier(apiServer.URL)(testContext(t), "valid-token", httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if err != nil {
		t.Fatalf("BearerTokenVerifier() failed: %v", err)
	}
	if info.UserID != "42" {
		t.Fatalf("UserID = %q, want 42", info.UserID)
	}
	if got := info.Extra[tokenInfoTripsyTokenKey]; got != "valid-token" {
		t.Fatalf("stored token = %v, want valid-token", got)
	}
}

func TestOAuthBearerTokenVerifierValidatesUserinfo(t *testing.T) {
	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer valid-oauth-token" {
			t.Errorf("Authorization = %q, want Bearer valid-oauth-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"user-uuid","email":"user@example.com"}`))
	}))
	defer userinfoServer.Close()

	info, err := OAuthBearerTokenVerifier(userinfoServer.URL)(testContext(t), "valid-oauth-token", httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if err != nil {
		t.Fatalf("OAuthBearerTokenVerifier() failed: %v", err)
	}
	if info.UserID != "user-uuid" {
		t.Fatalf("UserID = %q, want user-uuid", info.UserID)
	}
	if got := info.Extra[tokenInfoAuthSchemeKey]; got != "Bearer" {
		t.Fatalf("auth scheme = %v, want Bearer", got)
	}
}

func TestBearerTokenVerifierCachesRepeatedTokens(t *testing.T) {
	var calls atomic.Int32
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer apiServer.Close()

	verifier := BearerTokenVerifier(apiServer.URL)
	for i := 0; i < 3; i++ {
		if _, err := verifier(testContext(t), "repeat-token", httptest.NewRequest(http.MethodPost, "/mcp", nil)); err != nil {
			t.Fatalf("verifier() failed on call %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1 (cache should suppress repeats)", got)
	}
}

func TestActivitiesListSendsFilters(t *testing.T) {
	var called atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v2/trip/42/activities/" {
			t.Errorf("path = %s, want /v2/trip/42/activities/", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("fields"); got != "id,name" {
			t.Errorf("fields = %q, want id,name", got)
		}
		if got := query.Get("fields!"); got != "documents" {
			t.Errorf("fields! = %q, want documents", got)
		}
		if got := query.Get("activityType"); got != "museum" {
			t.Errorf("activityType = %q, want museum", got)
		}
		if got := query.Get("deleted"); got != "true" {
			t.Errorf("deleted = %q, want true", got)
		}
		if got := query.Get("updatedSince"); got != "2026-04-01T00:00:00Z" {
			t.Errorf("updatedSince = %q, want timestamp", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	})
	session, cleanup := connectTestSession(t, "test-token", handler)
	defer cleanup()

	res := callTool(t, session, "tripsy_activities_list", map[string]any{
		"trip_id":        "42",
		"fields":         []string{"name", "id"},
		"fields_exclude": []string{"documents"},
		"activity_type":  "museum",
		"deleted":        true,
		"updated_since":  "2026-04-01T00:00:00Z",
	})
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(res))
	}
	if called.Load() != 1 {
		t.Fatalf("handler called %d times, want 1", called.Load())
	}
	if got := structuredMap(t, res)["summary"]; got != "Activities" {
		t.Fatalf("summary = %v, want Activities", got)
	}
}

func TestToolsRequireTokenBeforeCallingAPI(t *testing.T) {
	var called atomic.Int32
	session, cleanup := connectTestSession(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		t.Errorf("API should not be called without a token")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer cleanup()

	res := callTool(t, session, "tripsy_trips_list", map[string]any{})
	if !res.IsError {
		t.Fatalf("expected tool error for missing token, got: %s", toolText(res))
	}
	if !strings.Contains(toolText(res), "not authenticated") {
		t.Fatalf("error text = %q, want not authenticated guidance", toolText(res))
	}
	if called.Load() != 0 {
		t.Fatalf("handler called %d times, want 0", called.Load())
	}
}

func TestRawRequestRejectsExternalURL(t *testing.T) {
	var called atomic.Int32
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		t.Errorf("API should not be called for an external raw URL")
	}))
	defer cleanup()

	res := callTool(t, session, "tripsy_raw_request", map[string]any{
		"method": "GET",
		"path":   "https://example.com/v1/me",
	})
	if !res.IsError {
		t.Fatalf("expected tool error for external raw URL, got: %s", toolText(res))
	}
	if !strings.Contains(toolText(res), "Tripsy API path") {
		t.Fatalf("error text = %q, want Tripsy API path guidance", toolText(res))
	}
	if called.Load() != 0 {
		t.Fatalf("handler called %d times, want 0", called.Load())
	}
}

func TestRawRequestRejectsWithheldCapabilities(t *testing.T) {
	session, cleanup := connectTestSession(t, "test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API should not be called for withheld MCP capabilities")
	}))
	defer cleanup()

	for _, tt := range []struct {
		path string
		want string
	}{
		{path: "/v1/emails", want: "email endpoints"},
		{path: "/v1/automation/emails/123", want: "inbox endpoints"},
		{path: "/v1/documents/123/get", want: "document endpoints"},
		{path: "/v1/trip/42/activity/9/documents", want: "document endpoints"},
		{path: "/v1/storage/uploads", want: "upload endpoints"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			res := callTool(t, session, "tripsy_raw_request", map[string]any{
				"method": "GET",
				"path":   tt.path,
			})
			if !res.IsError {
				t.Fatalf("expected tool error for %s, got: %s", tt.path, toolText(res))
			}
			if !strings.Contains(toolText(res), tt.want) {
				t.Fatalf("error text = %q, want %q", toolText(res), tt.want)
			}
		})
	}
}

func connectTestSession(t *testing.T, token string, handler http.Handler) (*mcp.ClientSession, func()) {
	return connectTestSessionOptions(t, token, handler, Options{})
}

func connectTestSessionOptions(t *testing.T, token string, handler http.Handler, opts Options) (*mcp.ClientSession, func()) {
	t.Helper()

	apiServer := httptest.NewServer(handler)
	opts.Version = firstNonEmpty(opts.Version, "test")
	server := NewWithClientOptions(api.NewClient(apiServer.URL, token), config.NewStore(t.TempDir()), opts)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(testContext(t), serverTransport, nil)
	if err != nil {
		apiServer.Close()
		t.Fatalf("server.Connect() failed: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(testContext(t), clientTransport, nil)
	if err != nil {
		serverSession.Close()
		apiServer.Close()
		t.Fatalf("client.Connect() failed: %v", err)
	}

	cleanup := func() {
		clientSession.Close()
		serverSession.Close()
		apiServer.Close()
	}
	return clientSession, cleanup
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(testContext(t), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%q) failed: %v", name, err)
	}
	return res
}

func findTool(tools []*mcp.Tool, name string) *mcp.Tool {
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	return nil
}

func toolSchemaString(t *testing.T, tool *mcp.Tool) string {
	t.Helper()
	data, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal tool schema for %s: %v", tool.Name, err)
	}
	return string(data)
}

func outputSchemaString(t *testing.T, tool *mcp.Tool) string {
	t.Helper()
	if tool.OutputSchema == nil {
		t.Fatalf("tool %s is missing output schema", tool.Name)
	}
	data, err := json.Marshal(tool.OutputSchema)
	if err != nil {
		t.Fatalf("marshal tool output schema for %s: %v", tool.Name, err)
	}
	return string(data)
}

func structuredMap(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	switch value := res.StructuredContent.(type) {
	case map[string]any:
		return value
	case json.RawMessage:
		var out map[string]any
		if err := json.Unmarshal(value, &out); err != nil {
			t.Fatalf("unmarshal structured content: %v", err)
		}
		return out
	case []byte:
		var out map[string]any
		if err := json.Unmarshal(value, &out); err != nil {
			t.Fatalf("unmarshal structured content: %v", err)
		}
		return out
	default:
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal structured content %T: %v", value, err)
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("unmarshal structured content %T: %v", value, err)
		}
		return out
	}
}

func toolText(res *mcp.CallToolResult) string {
	for _, content := range res.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			return text.Text
		}
	}
	return ""
}
