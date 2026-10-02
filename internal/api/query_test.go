package api

import (
	"net/url"
	"reflect"
	"testing"
)

func TestJoinFields(t *testing.T) {
	for _, test := range []struct {
		name   string
		fields []string
		want   string
	}{
		{name: "empty"},
		{name: "blank", fields: []string{" ", ",,"}},
		{name: "groups", fields: []string{" name, id, ", "id", " guests,name"}, want: "guests,id,name"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := JoinFields(test.fields); got != test.want {
				t.Fatalf("JoinFields() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTripListQuery(t *testing.T) {
	for _, test := range []struct {
		name  string
		query url.Values
		want  url.Values
	}{
		{name: "nil"},
		{name: "unrestricted", query: url.Values{}, want: url.Values{}},
		{
			name: "selected and excluded fields",
			query: url.Values{
				"fields":       {"name,id", "id"},
				"fields!":      {"owner, guests,documents", "has_dates,starts_at,ends_at, documents"},
				"updatedSince": {"2026-05-01T00:00:00Z"},
			},
			want: url.Values{
				"fields":       {"ends_at,guests,has_dates,id,name,owner,starts_at"},
				"fields!":      {"documents"},
				"updatedSince": {"2026-05-01T00:00:00Z"},
			},
		},
		{name: "required exclusions only", query: url.Values{"fields!": {"owner,guests"}}, want: url.Values{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := TripListQuery(test.query); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("TripListQuery() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTripDataQuery(t *testing.T) {
	if got := TripDataQuery(nil).Get("fields!"); got != "documents,emails" {
		t.Fatalf("nil query exclusions = %q, want documents,emails", got)
	}
	query := url.Values{"fields!": {"guests, documents", "guests"}, "fields": {"id"}}
	TripDataQuery(query)
	want := url.Values{"fields!": {"documents,emails,guests"}, "fields": {"id"}}
	if !reflect.DeepEqual(query, want) {
		t.Fatalf("query = %v, want %v", query, want)
	}
	if ExcludeFields(nil) != nil {
		t.Fatal("excluding no fields should preserve a nil query")
	}
}
