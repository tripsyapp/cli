package mcpserver

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const activityCategoryHint = "Supported activity_type slugs: concert, fit, general, kids, museum, note, relax, restaurant, shopping, theater, tour, event, meeting, bar, cafe, parking, amusementPark, aquarium, atm, bakery, bank, beach, brewery, campground, evCharger, fireStation, fitnessCenter, foodMarket, gasStation, hospital, laundry, library, marina, movieTheater, nationalPark, nightlife, park, pharmacy, police, postOffice, publicTransport, restroom, school, stadium, university, winery, zoo."

const transportationCategoryHint = "Supported transportation_type slugs: airplane, bike, bus, car, roadtrip, cruise, ferry, motorcycle, train, walk."

type emptyInput struct{}

type statusInput struct {
	Verbose bool `json:"verbose,omitempty" jsonschema:"When true and authenticated, include the current Tripsy profile in the status response."`
}

type dataInput struct {
	Data map[string]any `json:"data" jsonschema:"Object of Tripsy API fields to send."`
}

type tripCreateInput struct {
	Data          map[string]any `json:"data,omitempty" jsonschema:"Optional raw Tripsy API fields. Prefer the typed top-level fields when possible; values here are merged first."`
	Name          string         `json:"name,omitempty" jsonschema:"Trip name, such as Italy or Lisbon Birthday Weekend."`
	Timezone      string         `json:"timezone,omitempty" jsonschema:"Primary IANA timezone for the trip, such as Europe/Rome."`
	Description   string         `json:"description,omitempty" jsonschema:"Optional trip description."`
	StartsAt      string         `json:"starts_at,omitempty" jsonschema:"Trip start date as YYYY-MM-DD, for example 2026-06-01."`
	EndsAt        string         `json:"ends_at,omitempty" jsonschema:"Trip end date as YYYY-MM-DD, for example 2026-06-15."`
	CoverImageURL string         `json:"cover_image_url,omitempty" jsonschema:"Destination-specific direct Unsplash image URL. Must start with https://images.unsplash.com/photo-..., not an unsplash.com/photos page URL."`
	HasDates      *bool          `json:"has_dates,omitempty" jsonschema:"Whether the trip has explicit dates."`
	NumberOfDays  *int           `json:"number_of_days,omitempty" jsonschema:"Number of days for an undated trip."`
	Hidden        *bool          `json:"hidden,omitempty" jsonschema:"Whether the trip should be hidden."`
}

type activityCreateInput struct {
	TripID       string         `json:"trip_id" jsonschema:"Tripsy trip id."`
	Data         map[string]any `json:"data,omitempty" jsonschema:"Optional raw activity fields. Prefer the typed top-level fields when possible; values here are merged first."`
	Name         string         `json:"name,omitempty" jsonschema:"Activity name. Create one activity per actual stop, reservation, meal, tour, or experience."`
	ActivityType string         `json:"activity_type,omitempty" jsonschema:"Supported activity category slug. Use the most specific supported slug; do not invent values such as sightseeing."`
	StartsAt     string         `json:"starts_at,omitempty" jsonschema:"UTC ISO-8601 start timestamp for timed activities, for example 2026-06-03T09:00:00Z."`
	EndsAt       string         `json:"ends_at,omitempty" jsonschema:"UTC ISO-8601 end timestamp for timed activities, for example 2026-06-03T11:00:00Z."`
	Timezone     string         `json:"timezone,omitempty" jsonschema:"Local IANA timezone for the activity, such as Europe/Rome."`
	Address      string         `json:"address,omitempty" jsonschema:"Full address for map-ready location activities."`
	Latitude     *float64       `json:"latitude,omitempty" jsonschema:"Latitude for map-ready location activities."`
	Longitude    *float64       `json:"longitude,omitempty" jsonschema:"Longitude for map-ready location activities."`
	Description  string         `json:"description,omitempty" jsonschema:"Optional activity description."`
	Notes        string         `json:"notes,omitempty" jsonschema:"Optional notes."`
	Website      string         `json:"website,omitempty" jsonschema:"Optional website URL."`
	Price        *float64       `json:"price,omitempty" jsonschema:"Optional price."`
	Currency     string         `json:"currency,omitempty" jsonschema:"ISO currency code for price, such as EUR or USD."`
}

type hostingCreateInput struct {
	TripID      string         `json:"trip_id" jsonschema:"Tripsy trip id."`
	Data        map[string]any `json:"data,omitempty" jsonschema:"Optional raw hosting fields. Prefer the typed top-level fields when possible; values here are merged first."`
	Name        string         `json:"name,omitempty" jsonschema:"Hotel or lodging name. Use hostings for lodging rather than activities."`
	StartsAt    string         `json:"starts_at,omitempty" jsonschema:"UTC ISO-8601 check-in timestamp when known, for example 2026-06-01T14:00:00Z."`
	EndsAt      string         `json:"ends_at,omitempty" jsonschema:"UTC ISO-8601 check-out timestamp when known, for example 2026-06-05T11:00:00Z."`
	Timezone    string         `json:"timezone,omitempty" jsonschema:"Local IANA timezone for the lodging, such as Europe/Rome."`
	Address     string         `json:"address,omitempty" jsonschema:"Full lodging address for map display."`
	Latitude    *float64       `json:"latitude,omitempty" jsonschema:"Lodging latitude for map display."`
	Longitude   *float64       `json:"longitude,omitempty" jsonschema:"Lodging longitude for map display."`
	Description string         `json:"description,omitempty" jsonschema:"Optional lodging description."`
	RoomType    string         `json:"room_type,omitempty" jsonschema:"Optional room type."`
	RoomNumber  string         `json:"room_number,omitempty" jsonschema:"Optional room number."`
	Website     string         `json:"website,omitempty" jsonschema:"Optional website URL."`
	Price       *float64       `json:"price,omitempty" jsonschema:"Optional price."`
	Currency    string         `json:"currency,omitempty" jsonschema:"ISO currency code for price, such as EUR or USD."`
}

type transportationCreateInput struct {
	TripID               string         `json:"trip_id" jsonschema:"Tripsy trip id."`
	Data                 map[string]any `json:"data,omitempty" jsonschema:"Optional raw transportation fields. Prefer the typed top-level fields when possible; values here are merged first."`
	Name                 string         `json:"name,omitempty" jsonschema:"Transportation segment name. For flights, omit name unless the user provided one."`
	TransportationType   string         `json:"transportation_type,omitempty" jsonschema:"Supported transportation type slug. For flights, use airplane. For transfer activities, use roadtrip."`
	DepartureDescription string         `json:"departure_description,omitempty" jsonschema:"Departure location name or description. For flights, use the departure airport IATA code such as JFK or FCO. For transfers, include the real place name."`
	DepartureAt          string         `json:"departure_at,omitempty" jsonschema:"UTC ISO-8601 departure timestamp when known."`
	DepartureTimezone    string         `json:"departure_timezone,omitempty" jsonschema:"Departure IANA timezone."`
	DepartureAddress     string         `json:"departure_address,omitempty" jsonschema:"Full departure address. Required for transfer activities when known."`
	DepartureLatitude    *float64       `json:"departure_latitude,omitempty" jsonschema:"Departure latitude. Required for flight airports and transfer activities when known."`
	DepartureLongitude   *float64       `json:"departure_longitude,omitempty" jsonschema:"Departure longitude. Required for flight airports and transfer activities when known."`
	ArrivalDescription   string         `json:"arrival_description,omitempty" jsonschema:"Arrival location name or description. For flights, use the arrival airport IATA code such as JFK or FCO. For transfers, include the real place name."`
	ArrivalAt            string         `json:"arrival_at,omitempty" jsonschema:"UTC ISO-8601 arrival timestamp when known."`
	ArrivalTimezone      string         `json:"arrival_timezone,omitempty" jsonschema:"Arrival IANA timezone."`
	ArrivalAddress       string         `json:"arrival_address,omitempty" jsonschema:"Full arrival address. Required for transfer activities when known."`
	ArrivalLatitude      *float64       `json:"arrival_latitude,omitempty" jsonschema:"Arrival latitude. Required for flight airports and transfer activities when known."`
	ArrivalLongitude     *float64       `json:"arrival_longitude,omitempty" jsonschema:"Arrival longitude. Required for flight airports and transfer activities when known."`
	Description          string         `json:"description,omitempty" jsonschema:"Optional transportation description."`
	Notes                string         `json:"notes,omitempty" jsonschema:"Optional notes."`
	Company              string         `json:"company,omitempty" jsonschema:"Carrier, operator, or transfer company."`
	TransportNumber      string         `json:"transport_number,omitempty" jsonschema:"Flight, train, bus, or booking number when relevant."`
	VehicleDescription   string         `json:"vehicle_description,omitempty" jsonschema:"Vehicle description for cars, transfers, and roadtrips."`
	Price                *float64       `json:"price,omitempty" jsonschema:"Optional price."`
	Currency             string         `json:"currency,omitempty" jsonschema:"ISO currency code for price, such as EUR or USD."`
}

type idInput struct {
	ID string `json:"id" jsonschema:"Tripsy resource id."`
}

type tripIDInput struct {
	TripID string `json:"trip_id" jsonschema:"Tripsy trip id."`
}

type listInput struct {
	Fields        []string `json:"fields,omitempty" jsonschema:"Optional response field allow-list. Sent as the API fields query parameter."`
	FieldsExclude []string `json:"fields_exclude,omitempty" jsonschema:"Optional response field deny-list. Sent as the API fields! query parameter."`
	UpdatedSince  string   `json:"updated_since,omitempty" jsonschema:"Optional ISO-8601 timestamp for incremental list filtering."`
	Deleted       bool     `json:"deleted,omitempty" jsonschema:"When true, list deleted records where the endpoint supports it."`
}

type tripUpdateInput struct {
	ID   string         `json:"id" jsonschema:"Tripsy trip id."`
	Data map[string]any `json:"data" jsonschema:"Object of trip fields to update."`
}

type subresourceListInput struct {
	TripID             string   `json:"trip_id" jsonschema:"Tripsy trip id."`
	Fields             []string `json:"fields,omitempty" jsonschema:"Optional response field allow-list. Sent as the API fields query parameter."`
	FieldsExclude      []string `json:"fields_exclude,omitempty" jsonschema:"Optional response field deny-list. Sent as the API fields! query parameter."`
	UpdatedSince       string   `json:"updated_since,omitempty" jsonschema:"Optional ISO-8601 timestamp for incremental list filtering."`
	Deleted            bool     `json:"deleted,omitempty" jsonschema:"When true, list deleted records where the endpoint supports it."`
	ActivityType       string   `json:"activity_type,omitempty" jsonschema:"Optional activity category slug filter. Only used by activities."`
	TransportationType string   `json:"transportation_type,omitempty" jsonschema:"Optional transportation type slug filter. Only used by transportations."`
}

type tripResourceIDInput struct {
	TripID string `json:"trip_id" jsonschema:"Tripsy trip id."`
	ID     string `json:"id" jsonschema:"Tripsy subresource id."`
}

type tripResourceDataInput struct {
	TripID string         `json:"trip_id" jsonschema:"Tripsy trip id."`
	Data   map[string]any `json:"data" jsonschema:"Object of resource fields to create."`
}

type tripResourceUpdateInput struct {
	TripID string         `json:"trip_id" jsonschema:"Tripsy trip id."`
	ID     string         `json:"id" jsonschema:"Tripsy subresource id."`
	Data   map[string]any `json:"data" jsonschema:"Object of resource fields to update."`
}

type rawRequestInput struct {
	Method string            `json:"method" jsonschema:"HTTP method such as GET, POST, PATCH, or DELETE."`
	Path   string            `json:"path" jsonschema:"Tripsy API path such as /v1/me or /v1/trips."`
	Query  map[string]string `json:"query,omitempty" jsonschema:"Optional query parameters."`
	Data   map[string]any    `json:"data,omitempty" jsonschema:"Optional JSON object request body."`
}

type resourceSpec struct {
	Prefix      string
	Title       string
	PluralTitle string
	ListPath    string
	DetailPath  string
	FilterName  string
	FilterParam string
	FilterHint  string

	Description  string
	CreateAdvice string
	ExcludeData  bool
	SkipCreate   bool
}

type itineraryGuidanceInput struct{}

func (s *service) status(ctx context.Context, req *mcp.CallToolRequest, in statusInput) (*mcp.CallToolResult, any, error) {
	client := s.clientForRequest(req)
	data := map[string]any{
		"api_base":         client.BaseURL,
		"auth_backend":     s.store.AuthBackendName(),
		"config_dir":       s.store.Dir,
		"credentials_path": s.store.CredentialsPath(),
		"has_token":        strings.TrimSpace(client.Token) != "",
		"remote_token":     req != nil && req.Extra != nil && req.Extra.TokenInfo != nil,
	}
	if strings.TrimSpace(client.Token) != "" {
		resp, err := client.Request(ctx, "GET", "/v1/me", nil, nil)
		if err != nil {
			data["api_check"] = err.Error()
		} else {
			data["api_check"] = "ok"
			if in.Verbose {
				data["me"] = resp.Data
			}
		}
	}
	return nil, map[string]any{"summary": "Tripsy MCP status", "data": data}, nil
}

func (s *service) rawRequest(ctx context.Context, req *mcp.CallToolRequest, in rawRequestInput) (*mcp.CallToolResult, any, error) {
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method == "" {
		return nil, nil, fmt.Errorf("method is required")
	}
	switch method {
	case "GET", "POST", "PATCH", "PUT", "DELETE":
	default:
		return nil, nil, fmt.Errorf("unsupported method %q; expected GET, POST, PATCH, PUT, or DELETE", method)
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, nil, fmt.Errorf("path must be a Tripsy API path beginning with /")
	}
	if err := allowRawRequestPath(path); err != nil {
		return nil, nil, err
	}
	query := url.Values{}
	for key, value := range in.Query {
		query.Set(key, value)
	}
	var body any
	if len(in.Data) > 0 {
		body = in.Data
	}
	return toolOutput(s.do(ctx, req, method, path, query, body, "Raw Tripsy API response"))
}

func (s *service) itineraryGuidance(context.Context, *mcp.CallToolRequest, itineraryGuidanceInput) (*mcp.CallToolResult, any, error) {
	rules := []string{
		"Create the trip first with name, timezone, starts_at, ends_at, and cover_image_url when planning dated travel.",
		"For leisure or destination trips, choose a destination-specific direct Unsplash image URL for cover_image_url.",
		"cover_image_url must be an https://images.unsplash.com/photo-... URL, not an unsplash.com/photos page URL.",
		"Create one Tripsy item per actual stop, reservation, meal, tour, lodging, or transportation segment.",
		"Use activities for stops, meals, tours, events, and experiences; choose the most specific supported activity_type slug.",
		"Use hostings for hotels and lodging, with address, latitude, and longitude when known.",
		"Use transportations for point-to-point movement.",
		"For flights, create a transportation with transportation_type airplane, set departure_description and arrival_description to the airport IATA codes, include the airports' departure/arrival latitudes and longitudes, and omit name unless the user provided one.",
		"For transfer activities, create a transportation with transportation_type roadtrip and fill departure and arrival name/description, address, latitude, and longitude.",
		"Use exact UTC ISO-8601 timestamps for timed items and set the relevant local timezone.",
		"Add address, latitude, and longitude for map-relevant activities, hostings, and transportation endpoints.",
	}
	doNot := []string{
		"Do not use unsplash.com/photos/... as cover_image_url.",
		"Do not create one activity named Day 1 itinerary or similar that contains multiple stops.",
		"Do not put hotels or lodging into activities.",
		"Do not put transfers into activities.",
		"Do not omit coordinates when a location is known.",
		"Do not use unsupported activity_type values such as sightseeing.",
	}
	example := map[string]any{
		"trip": map[string]any{
			"name":            "Rome",
			"timezone":        "Europe/Rome",
			"starts_at":       "2026-06-01",
			"ends_at":         "2026-06-05",
			"cover_image_url": "https://images.unsplash.com/photo-1529260830199-42c24126f198?ixlib=rb-4.1.0",
		},
		"hosting": map[string]any{
			"name":      "Hotel Eden",
			"starts_at": "2026-06-01T14:00:00Z",
			"ends_at":   "2026-06-05T11:00:00Z",
			"timezone":  "Europe/Rome",
			"address":   "Via Ludovisi 49, 00187 Rome, Italy",
			"latitude":  41.9081,
			"longitude": 12.4882,
		},
		"activity": map[string]any{
			"name":          "Colosseum Tour",
			"activity_type": "tour",
			"starts_at":     "2026-06-03T09:00:00Z",
			"ends_at":       "2026-06-03T11:00:00Z",
			"timezone":      "Europe/Rome",
			"address":       "Piazza del Colosseo, 1, 00184 Rome, Italy",
			"latitude":      41.8902,
			"longitude":     12.4922,
		},
		"transfer": map[string]any{
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
		},
	}
	data := map[string]any{
		"rules":           rules,
		"do_not":          doNot,
		"example_payload": example,
	}
	return nil, map[string]any{"summary": "Tripsy itinerary guidance", "data": data}, nil
}

func allowRawRequestPath(apiPath string) error {
	cleaned := path.Clean("/" + strings.TrimLeft(apiPath, "/"))
	blocked := map[string]string{
		"/v1/emails":            "email",
		"/v1/automation/emails": "inbox",
		"/v1/documents":         "document",
		"/v1/storage/uploads":   "upload",
	}
	for prefix, name := range blocked {
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") {
			return fmt.Errorf("%s endpoints are not exposed by the Tripsy MCP server yet", name)
		}
	}
	if strings.Contains(cleaned, "/documents") {
		return fmt.Errorf("document endpoints are not exposed by the Tripsy MCP server yet")
	}
	return nil
}

func (s *service) meShow(ctx context.Context, req *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
	return toolOutput(s.do(ctx, req, "GET", "/v1/me", nil, nil, "Current user"))
}

func (s *service) meUpdate(ctx context.Context, req *mcp.CallToolRequest, in dataInput) (*mcp.CallToolResult, any, error) {
	if len(in.Data) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, req, "PATCH", "/v1/me", nil, in.Data, "Current user updated"))
}

func (s *service) tripsList(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolOutput(s.do(ctx, req, "GET", "/v1/trips", tripDataQuery(listQuery(in)), nil, "Trips"))
}

func (s *service) tripShow(ctx context.Context, req *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, req, "GET", "/v1/trips/"+apiPathSegment(in.ID), tripDataQuery(nil), nil, "Trip "+in.ID))
}

func (s *service) tripCreate(ctx context.Context, req *mcp.CallToolRequest, in tripCreateInput) (*mcp.CallToolResult, any, error) {
	payload := tripCreatePayload(in)
	if len(payload) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, req, "POST", "/v1/trips", tripDataQuery(nil), payload, "Trip created"))
}

func (s *service) tripUpdate(ctx context.Context, req *mcp.CallToolRequest, in tripUpdateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	if len(in.Data) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, req, "PATCH", "/v1/trips/"+apiPathSegment(in.ID), tripDataQuery(nil), in.Data, "Trip updated"))
}

func (s *service) tripDelete(ctx context.Context, req *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	return toolOutput(s.do(ctx, req, "DELETE", "/v1/trips/"+apiPathSegment(in.ID), tripDataQuery(nil), nil, "Trip deleted"))
}

func (s *service) activityCreate(ctx context.Context, req *mcp.CallToolRequest, in activityCreateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.TripID) == "" {
		return nil, nil, fmt.Errorf("trip_id is required")
	}
	payload := activityCreatePayload(in)
	if len(payload) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, req, "POST", "/v1/trip/"+apiPathSegment(in.TripID)+"/activities", tripDataQuery(nil), payload, "Activity created"))
}

func (s *service) hostingCreate(ctx context.Context, req *mcp.CallToolRequest, in hostingCreateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.TripID) == "" {
		return nil, nil, fmt.Errorf("trip_id is required")
	}
	payload := hostingCreatePayload(in)
	if len(payload) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, req, "POST", "/v1/trip/"+apiPathSegment(in.TripID)+"/hostings", tripDataQuery(nil), payload, "Hosting created"))
}

func (s *service) transportationCreate(ctx context.Context, req *mcp.CallToolRequest, in transportationCreateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.TripID) == "" {
		return nil, nil, fmt.Errorf("trip_id is required")
	}
	payload := transportationCreatePayload(in)
	if len(payload) == 0 {
		return nil, nil, fmt.Errorf("data is required")
	}
	return toolOutput(s.do(ctx, req, "POST", "/v1/trip/"+apiPathSegment(in.TripID)+"/transportations", tripDataQuery(nil), payload, "Transportation created"))
}

func tripCreatePayload(in tripCreateInput) map[string]any {
	payload := cloneData(in.Data)
	setString(payload, "name", in.Name)
	setString(payload, "timezone", in.Timezone)
	setString(payload, "description", in.Description)
	setString(payload, "starts_at", in.StartsAt)
	setString(payload, "ends_at", in.EndsAt)
	setString(payload, "cover_image_url", in.CoverImageURL)
	setBool(payload, "has_dates", in.HasDates)
	setInt(payload, "number_of_days", in.NumberOfDays)
	setBool(payload, "hidden", in.Hidden)
	return payload
}

func activityCreatePayload(in activityCreateInput) map[string]any {
	payload := cloneData(in.Data)
	setString(payload, "name", in.Name)
	setString(payload, "activity_type", in.ActivityType)
	setString(payload, "starts_at", in.StartsAt)
	setString(payload, "ends_at", in.EndsAt)
	setString(payload, "timezone", in.Timezone)
	setString(payload, "address", in.Address)
	setFloat(payload, "latitude", in.Latitude)
	setFloat(payload, "longitude", in.Longitude)
	setString(payload, "description", in.Description)
	setString(payload, "notes", in.Notes)
	setString(payload, "website", in.Website)
	setFloat(payload, "price", in.Price)
	setString(payload, "currency", in.Currency)
	return payload
}

func hostingCreatePayload(in hostingCreateInput) map[string]any {
	payload := cloneData(in.Data)
	setString(payload, "name", in.Name)
	setString(payload, "starts_at", in.StartsAt)
	setString(payload, "ends_at", in.EndsAt)
	setString(payload, "timezone", in.Timezone)
	setString(payload, "address", in.Address)
	setFloat(payload, "latitude", in.Latitude)
	setFloat(payload, "longitude", in.Longitude)
	setString(payload, "description", in.Description)
	setString(payload, "room_type", in.RoomType)
	setString(payload, "room_number", in.RoomNumber)
	setString(payload, "website", in.Website)
	setFloat(payload, "price", in.Price)
	setString(payload, "currency", in.Currency)
	return payload
}

func transportationCreatePayload(in transportationCreateInput) map[string]any {
	payload := cloneData(in.Data)
	setString(payload, "name", in.Name)
	setString(payload, "transportation_type", in.TransportationType)
	setString(payload, "departure_description", in.DepartureDescription)
	setString(payload, "departure_at", in.DepartureAt)
	setString(payload, "departure_timezone", in.DepartureTimezone)
	setString(payload, "departure_address", in.DepartureAddress)
	setFloat(payload, "departure_latitude", in.DepartureLatitude)
	setFloat(payload, "departure_longitude", in.DepartureLongitude)
	setString(payload, "arrival_description", in.ArrivalDescription)
	setString(payload, "arrival_at", in.ArrivalAt)
	setString(payload, "arrival_timezone", in.ArrivalTimezone)
	setString(payload, "arrival_address", in.ArrivalAddress)
	setFloat(payload, "arrival_latitude", in.ArrivalLatitude)
	setFloat(payload, "arrival_longitude", in.ArrivalLongitude)
	setString(payload, "description", in.Description)
	setString(payload, "notes", in.Notes)
	setString(payload, "company", in.Company)
	setString(payload, "transport_number", in.TransportNumber)
	setString(payload, "vehicle_description", in.VehicleDescription)
	setFloat(payload, "price", in.Price)
	setString(payload, "currency", in.Currency)
	return payload
}

func cloneData(data map[string]any) map[string]any {
	payload := make(map[string]any, len(data))
	for key, value := range data {
		payload[key] = value
	}
	return payload
}

func setString(payload map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		payload[key] = value
	}
}

func setFloat(payload map[string]any, key string, value *float64) {
	if value != nil {
		payload[key] = *value
	}
}

func setBool(payload map[string]any, key string, value *bool) {
	if value != nil {
		payload[key] = *value
	}
}

func setInt(payload map[string]any, key string, value *int) {
	if value != nil {
		payload[key] = *value
	}
}

func (s *service) registerResource(server *mcp.Server, spec resourceSpec) {
	pluralTitle := firstNonEmpty(spec.PluralTitle, spec.Title+"s")
	filterText := ""
	if spec.FilterHint != "" {
		filterText = " " + spec.FilterHint
	}
	addTool(server, toolName("tripsy", spec.Prefix, "list"), "List "+pluralTitle, spec.Description+" Supports common list filters."+filterText, readOnly(), func(ctx context.Context, req *mcp.CallToolRequest, in subresourceListInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		query := subresourceListQuery(in)
		if spec.FilterParam != "" {
			switch spec.FilterName {
			case "activity_type":
				if in.ActivityType != "" {
					query.Set(spec.FilterParam, in.ActivityType)
				}
			case "transportation_type":
				if in.TransportationType != "" {
					query.Set(spec.FilterParam, in.TransportationType)
				}
			}
		}
		return toolOutput(s.do(ctx, req, "GET", fmt.Sprintf(spec.ListPath, apiPathSegment(in.TripID)), spec.responseQuery(query), nil, pluralTitle))
	})
	addTool(server, toolName("tripsy", spec.Prefix, "show"), "Show "+spec.Title, "Fetch one Tripsy "+strings.ToLower(spec.Title)+" by id.", readOnly(), func(ctx context.Context, req *mcp.CallToolRequest, in tripResourceIDInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf("id is required")
		}
		return toolOutput(s.do(ctx, req, "GET", fmt.Sprintf(spec.DetailPath, apiPathSegment(in.TripID), apiPathSegment(in.ID)), spec.responseQuery(nil), nil, spec.Title+" "+in.ID))
	})
	if !spec.SkipCreate {
		addTool(server, toolName("tripsy", spec.Prefix, "create"), "Create "+spec.Title, "Create a Tripsy "+strings.ToLower(spec.Title)+". "+spec.CreateAdvice, additive(), func(ctx context.Context, req *mcp.CallToolRequest, in tripResourceDataInput) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(in.TripID) == "" {
				return nil, nil, fmt.Errorf("trip_id is required")
			}
			if len(in.Data) == 0 {
				return nil, nil, fmt.Errorf("data is required")
			}
			return toolOutput(s.do(ctx, req, "POST", fmt.Sprintf(spec.ListPath, apiPathSegment(in.TripID)), spec.responseQuery(nil), in.Data, spec.Title+" created"))
		})
	}
	addTool(server, toolName("tripsy", spec.Prefix, "update"), "Update "+spec.Title, "Update a Tripsy "+strings.ToLower(spec.Title)+" by id.", idempotentWrite(), func(ctx context.Context, req *mcp.CallToolRequest, in tripResourceUpdateInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf("id is required")
		}
		if len(in.Data) == 0 {
			return nil, nil, fmt.Errorf("data is required")
		}
		return toolOutput(s.do(ctx, req, "PATCH", fmt.Sprintf(spec.DetailPath, apiPathSegment(in.TripID), apiPathSegment(in.ID)), spec.responseQuery(nil), in.Data, spec.Title+" updated"))
	})
	addTool(server, toolName("tripsy", spec.Prefix, "delete"), "Delete "+spec.Title, "Delete a Tripsy "+strings.ToLower(spec.Title)+" by id.", destructive(), func(ctx context.Context, req *mcp.CallToolRequest, in tripResourceIDInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.TripID) == "" {
			return nil, nil, fmt.Errorf("trip_id is required")
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, nil, fmt.Errorf("id is required")
		}
		return toolOutput(s.do(ctx, req, "DELETE", fmt.Sprintf(spec.DetailPath, apiPathSegment(in.TripID), apiPathSegment(in.ID)), spec.responseQuery(nil), nil, spec.Title+" deleted"))
	})
}

func (s *service) collaboratorsList(ctx context.Context, req *mcp.CallToolRequest, in tripIDInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.TripID) == "" {
		return nil, nil, fmt.Errorf("trip_id is required")
	}
	return toolOutput(s.do(ctx, req, "GET", "/v1/trip/"+apiPathSegment(in.TripID)+"/collaborators", nil, nil, "Collaborators"))
}

func apiPathSegment(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}

func listQuery(in listInput) url.Values {
	query := url.Values{}
	addListQuery(query, in.Fields, in.FieldsExclude, in.UpdatedSince, in.Deleted)
	return query
}

func subresourceListQuery(in subresourceListInput) url.Values {
	query := url.Values{}
	addListQuery(query, in.Fields, in.FieldsExclude, in.UpdatedSince, in.Deleted)
	return query
}

func addListQuery(query url.Values, fields, fieldsExclude []string, updatedSince string, deleted bool) {
	if deleted {
		query.Set("deleted", "true")
	}
	if updatedSince != "" {
		query.Set("updatedSince", updatedSince)
	}
	if len(fields) > 0 {
		query.Set("fields", joinFields(fields))
	}
	addFieldsExclude(query, fieldsExclude)
}

func joinFields(fields []string) string {
	normalized := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		for _, part := range strings.Split(field, ",") {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				normalized = append(normalized, part)
			}
		}
	}
	sort.Strings(normalized)
	return strings.Join(normalized, ",")
}

var defaultTripDataFieldsExclude = []string{"documents", "emails"}

func (spec resourceSpec) responseQuery(query url.Values) url.Values {
	if !spec.ExcludeData {
		return query
	}
	return tripDataQuery(query)
}

func tripDataQuery(query url.Values) url.Values {
	if query == nil {
		query = url.Values{}
	}
	addFieldsExclude(query, defaultTripDataFieldsExclude)
	return query
}

func addFieldsExclude(query url.Values, fields []string) {
	if len(fields) == 0 {
		return
	}
	values := append([]string{}, query["fields!"]...)
	values = append(values, fields...)
	if joined := joinFields(values); joined != "" {
		query.Set("fields!", joined)
	}
}

func toolOutput(value any, err error) (*mcp.CallToolResult, any, error) {
	return nil, value, err
}
