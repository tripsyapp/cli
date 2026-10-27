package mcpserver

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/tripsyapp/cli/internal/api"
)

func transportationToolInputSchema[In any](name string) any {
	switch name {
	case "tripsy_transportations_create", "tripsy_transportations_update", "tripsy_transportations_list":
	default:
		return nil
	}
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	typeSchema := &jsonschema.Schema{Type: "string", Description: api.TransportationTypeGuidance}
	for _, slug := range api.TransportationTypes() {
		typeSchema.Enum = append(typeSchema.Enum, slug)
	}
	if field := schema.Properties["transportation_type"]; field != nil {
		field.Enum = typeSchema.Enum
		field.Description += " " + api.TransportationTypeGuidance
	}
	if data := schema.Properties["data"]; data != nil {
		data.Description += " " + api.TransportationTypeGuidance
		if name == "tripsy_transportations_update" {
			data.Properties = map[string]*jsonschema.Schema{"transportation_type": typeSchema}
		}
	}
	return schema
}
