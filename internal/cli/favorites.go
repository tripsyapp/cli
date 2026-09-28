package cli

import (
	"context"
	"fmt"

	"github.com/tripsyapp/cli/internal/output"
)

func (a *app) guests(ctx context.Context, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "favorites" || (len(args) > 1 && (len(args) != 2 || args[1] != "list")) {
		return usageError("usage: tripsy guests favorites [list]")
	}
	resp, err := a.client.RequestAllPages(ctx, "GET", "/v1/guests/favorites", nil, nil)
	if err != nil {
		return err
	}
	rows := make([]any, 0, len(results(resp.Data)))
	for _, item := range results(resp.Data) {
		entry := objectMap(item)
		user := objectMap(entry["favorite_user"])
		status := "confirmed"
		if entry["pending"] == true {
			status = "pending"
		}
		rows = append(rows, map[string]any{"user_id": user["id"], "name": user["name"], "email": user["email"], "status": status})
	}
	return a.render(output.Result{
		Data: resp.Data, Summary: fmt.Sprintf("%d favorite guests and pending invitations", len(rows)),
		Human: formatObjects("Favorite guests (user ID, name, email, status)", rows, "user_id", "name", "email", "status"),
	})
}
