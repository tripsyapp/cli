package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
)

// Keep these slugs in sync with iOS ActivityType.transportation. Core's
// Transportation.transportation_type is an unrestricted string field.
const transportationTypeSlugs = "airplane, bike, bus, car, roadtrip, cruise, ferry, motorcycle, subway, train, transfer, walk"

const TransportationTypeGuidance = "Supported transportation_type slugs: " + transportationTypeSlugs + ". Labels: airplane = Flight; bike = Bike; bus = Bus; car = Car Rental; roadtrip = Drive / Road Trip; cruise = Cruise; ferry = Ferry; motorcycle = Motorcycle; subway = Subway; train = Train; transfer = Transfer; walk = Walk. Use only these exact slugs; custom categories are not supported for transportations."

// TransportationTypes returns a fresh copy of the supported transportation slugs.
func TransportationTypes() []string {
	return strings.Split(transportationTypeSlugs, ", ")
}

func validateTransportationType(value string) error {
	if !slices.Contains(TransportationTypes(), value) {
		return fmt.Errorf("invalid transportation_type %q; allowed values: %s", value, transportationTypeSlugs)
	}
	return nil
}

func isTransportationPath(apiPath string) bool {
	parts := strings.Split(strings.Trim(path.Clean(apiPath), "/"), "/")
	if len(parts) < 4 || (parts[0] != "v1" && parts[0] != "v2") || parts[1] != "trip" || parts[2] == "" {
		return false
	}
	return (len(parts) == 4 && parts[3] == "transportations") ||
		(len(parts) == 5 && parts[3] == "transportation" && parts[4] != "")
}

func validateTransportationQuery(target string) error {
	u, err := url.Parse(target)
	if err != nil || !isTransportationPath(u.EscapedPath()) {
		return err
	}
	for _, value := range u.Query()["transportationType"] {
		if err := validateTransportationType(value); err != nil {
			return err
		}
	}
	return nil
}

func validateTransportationFields(method, target string, fields map[string]json.RawMessage) error {
	switch strings.ToUpper(method) {
	case "POST", "PATCH", "PUT":
	default:
		return nil
	}
	u, err := url.Parse(target)
	if err != nil || !isTransportationPath(u.EscapedPath()) {
		return err
	}
	raw, present := fields["transportation_type"]
	if !present {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	slug, ok := value.(string)
	if !ok {
		return fmt.Errorf("transportation_type must be a string; allowed values: %s", transportationTypeSlugs)
	}
	return validateTransportationType(slug)
}
