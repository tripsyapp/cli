package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tripsyapp/cli/internal/guests"
	"github.com/tripsyapp/cli/internal/output"
)

var collaboratorPermissionFields = []string{
	"can_edit", "can_add_guests", "can_see_expenses", "can_edit_expenses",
	"can_see_documents", "can_edit_documents", "is_travelling", "receive_notifications",
}

var invitationPermissionFields = []string{
	"title", "read_only", "can_add_guests", "can_see_expenses", "can_edit_expenses",
	"can_see_documents", "can_edit_documents", "is_travelling",
}

func (a *app) collaborators(ctx context.Context, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	action := "list"
	if len(args) > 0 {
		switch args[0] {
		case "list", "invite", "update", "delete":
			action, args = args[0], args[1:]
		}
	}
	fs, err := parseFlags(args)
	if err != nil {
		return err
	}
	fields := []string{"trip", "in"}
	switch action {
	case "invite":
		fields = append(fields, "email", "data", "set")
		fields = append(fields, invitationPermissionFields...)
	case "update":
		fields = append(fields, "data", "set")
		fields = append(fields, collaboratorPermissionFields...)
	}
	if err := validateGuestFlags(fs, fields); err != nil {
		return err
	}
	tripID := strings.TrimSpace(fs.TripID())
	// Preserve the original `collaborators TRIP_ID` and `collaborators --trip` forms.
	if action == "list" && tripID == "" && len(fs.positionals) == 1 {
		tripID = strings.TrimSpace(fs.positionals[0])
		fs.positionals = nil
	}
	if tripID == "" {
		return usageError("collaborators %s requires --trip", action)
	}
	if action == "list" || action == "invite" {
		if len(fs.positionals) != 0 {
			return usageError("unexpected arguments for collaborators %s", action)
		}
	} else if len(fs.positionals) != 1 || strings.TrimSpace(fs.positionals[0]) == "" {
		return usageError("collaborators %s requires one user id", action)
	}
	switch action {
	case "list":
		resp, err := a.client.RequestAllPages(ctx, "GET", "/v1/trip/"+apiPathSegment(tripID)+"/collaborators", nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: fmt.Sprintf("%d collaborators", len(results(resp.Data))), Human: formatObjects("Collaborators", resp.Data, "id", "name", "email", "joined", "permissions")})
	case "invite":
		email := strings.TrimSpace(fs.String("email"))
		if email == "" {
			return usageError("collaborators invite requires --email")
		}
		var permissions guests.InvitePermissions
		if err := guestPermissionsPayload(fs, invitationPermissionFields, &permissions); err != nil {
			return err
		}
		payload := map[string]any{"trip_id": tripID, "invited_user_email": email, "permissions": permissions}
		resp, err := a.client.Request(ctx, "POST", "/v1/guests/invite", nil, payload)
		if err != nil {
			return err
		}
		summary := "Invitation request processed. Check collaborators for membership or pending status; success does not confirm a guest was added."
		return a.render(output.Result{Data: resp.Data, Summary: summary, Human: summary + "\n" + formatFullObject("API response", resp.Data)})
	case "update":
		var permissions guests.UpdatePermissions
		if err := guestPermissionsPayload(fs, collaboratorPermissionFields, &permissions); err != nil {
			return err
		}
		if permissions.Empty() {
			return usageError("collaborators update requires at least one permission field")
		}
		userID := strings.TrimSpace(fs.positionals[0])
		if userID == "me" {
			var err error
			userID, err = a.currentUserID(ctx)
			if err != nil {
				return err
			}
		}
		resp, err := a.client.Request(ctx, "PATCH", "/v1/trip/"+apiPathSegment(tripID)+"/collaborator/"+apiPathSegment(userID)+"/permissions", nil, permissions)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Collaborator permissions updated", Human: formatFullObject("Collaborator permissions updated", resp.Data)})
	case "delete":
		resp, err := a.client.Request(ctx, "DELETE", "/v1/trip/"+apiPathSegment(tripID)+"/collaborator/"+apiPathSegment(fs.positionals[0]), nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: requestData(resp), Summary: "Collaborator removal request processed", Human: "Collaborator removal request processed.\n"})
	}
	return nil
}

func validateGuestFlags(fs *flagSet, fields []string) error {
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[strings.ReplaceAll(field, "_", "-")] = true
	}
	for flag := range fs.values {
		if !allowed[flag] {
			return usageError("unsupported flag --%s", flag)
		}
	}
	return nil
}

// Both --data and --set contain permission fields, not the outer request body.
func guestPermissionsPayload(fs *flagSet, fields []string, target any) error {
	payload, err := buildPayload(fs, fields)
	if err != nil {
		return err
	}
	for field, value := range payload {
		if field == "title" {
			if _, ok := value.(string); !ok {
				return usageError("permission title must be a string")
			}
		} else if _, ok := value.(bool); !ok {
			return usageError("permission %s must be true or false", field)
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return usageError("invalid permissions: %v", err)
	}
	return nil
}
