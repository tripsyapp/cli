package api

import (
	"net/url"
	"slices"
	"strings"
)

// JoinFields normalizes comma-separated field groups into a sorted, unique list.
func JoinFields(fields []string) string {
	normalized := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		for part := range strings.SplitSeq(field, ",") {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				normalized = append(normalized, part)
			}
		}
	}
	slices.Sort(normalized)
	return strings.Join(normalized, ",")
}

// ExcludeFields adds fields to the API projection exclusions, allocating query
// when needed. Existing query values are updated in place.
func ExcludeFields(query url.Values, fields ...string) url.Values {
	if len(fields) == 0 {
		return query
	}
	if query == nil {
		query = url.Values{}
	}
	if joined := JoinFields(slices.Concat(query["fields!"], fields)); joined != "" {
		query.Set("fields!", joined)
	}
	return query
}

// TripDataQuery omits heavyweight attachments from trip and itinerary responses.
func TripDataQuery(query url.Values) url.Values {
	return ExcludeFields(query, "documents", "emails")
}

// TripListQuery keeps the fields needed to filter travel status and display dates
// available, even when the caller selects or excludes specific fields.
func TripListQuery(query url.Values) url.Values {
	fields := []string{"owner", "guests", "has_dates", "starts_at", "ends_at"}
	if query.Get("fields") != "" {
		query.Set("fields", JoinFields(slices.Concat(query["fields"], fields)))
	}
	removeFieldsExclude(query, fields)
	return query
}

func removeFieldsExclude(query url.Values, fields []string) {
	if query.Get("fields!") == "" {
		return
	}
	blocked := map[string]bool{}
	for _, field := range fields {
		blocked[field] = true
	}
	kept := make([]string, 0, len(query["fields!"]))
	for _, value := range query["fields!"] {
		for part := range strings.SplitSeq(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" && !blocked[part] {
				kept = append(kept, part)
			}
		}
	}
	if joined := JoinFields(kept); joined != "" {
		query.Set("fields!", joined)
	} else {
		query.Del("fields!")
	}
}
