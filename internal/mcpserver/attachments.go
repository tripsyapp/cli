package mcpserver

import (
	"fmt"
	"strings"
)

// attachmentParentInput identifies a trip or one exact itinerary object within it.
type attachmentParentInput struct {
	TripID     string `json:"trip_id" jsonschema:"Trip id. The API checks access and document permissions."`
	ParentType string `json:"parent_type,omitempty" jsonschema:"trip (default), activity, hosting, or transportation. Trip lists aggregate attachments from the whole trip."`
	ParentID   string `json:"parent_id,omitempty" jsonschema:"Required for activity, hosting, or transportation. Omit for trip."`
}

type attachmentShowInput struct {
	attachmentParentInput
	ID string `json:"id" jsonschema:"Attachment id from the corresponding list tool."`
}

func attachmentPath(in attachmentParentInput, version, resource string) (string, error) {
	tripID := strings.TrimSpace(in.TripID)
	if tripID == "" {
		return "", fmt.Errorf("trip_id is required")
	}
	parentType := strings.TrimSpace(in.ParentType)
	if parentType == "" {
		parentType = "trip"
	}
	parentID := strings.TrimSpace(in.ParentID)
	path := "/" + version + "/trip/" + apiPathSegment(tripID)
	switch parentType {
	case "trip":
		if parentID != "" {
			return "", fmt.Errorf("omit parent_id for a trip parent")
		}
	case "activity", "hosting", "transportation":
		if parentID == "" {
			return "", fmt.Errorf("parent_id is required for %s", parentType)
		}
		path += "/" + parentType + "/" + apiPathSegment(parentID)
	default:
		return "", fmt.Errorf("parent_type must be trip, activity, hosting, or transportation")
	}
	return path + "/" + resource, nil
}
