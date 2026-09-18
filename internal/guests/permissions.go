// Package guests defines the permission payloads shared by the CLI and MCP.
package guests

// Pointers distinguish an omitted permission from an explicit false value.
type UpdatePermissions struct {
	CanEdit              *bool `json:"can_edit,omitempty" jsonschema:"Allow editing the trip."`
	CanAddGuests         *bool `json:"can_add_guests,omitempty" jsonschema:"Allow managing trip guests."`
	CanSeeExpenses       *bool `json:"can_see_expenses,omitempty" jsonschema:"Allow viewing expenses."`
	CanEditExpenses      *bool `json:"can_edit_expenses,omitempty" jsonschema:"Allow editing expenses."`
	CanSeeDocuments      *bool `json:"can_see_documents,omitempty" jsonschema:"Allow viewing documents."`
	CanEditDocuments     *bool `json:"can_edit_documents,omitempty" jsonschema:"Allow editing documents."`
	ReceiveNotifications *bool `json:"receive_notifications,omitempty" jsonschema:"Receive trip notifications. Send this field alone when changing your own notification preference."`
}

func (p UpdatePermissions) Empty() bool {
	return p == (UpdatePermissions{})
}

// Invitations use read_only rather than the collaborator update's can_edit.
type InvitePermissions struct {
	Title            string `json:"title,omitempty" jsonschema:"Optional invitation title."`
	ReadOnly         *bool  `json:"read_only,omitempty" jsonschema:"Invite with read-only trip access."`
	CanAddGuests     *bool  `json:"can_add_guests,omitempty" jsonschema:"Allow managing trip guests."`
	CanSeeExpenses   *bool  `json:"can_see_expenses,omitempty" jsonschema:"Allow viewing expenses."`
	CanEditExpenses  *bool  `json:"can_edit_expenses,omitempty" jsonschema:"Allow editing expenses."`
	CanSeeDocuments  *bool  `json:"can_see_documents,omitempty" jsonschema:"Allow viewing documents."`
	CanEditDocuments *bool  `json:"can_edit_documents,omitempty" jsonschema:"Allow editing documents."`
	IsTravelling     *bool  `json:"is_travelling,omitempty" jsonschema:"True for travelling; false for following."`
}
