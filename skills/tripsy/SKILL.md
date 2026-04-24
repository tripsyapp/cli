# Tripsy CLI Agent Skill

Use this skill when an agent needs to inspect, create, update, or organize Tripsy data through the local `tripsy` CLI.

The CLI talks to the public Tripsy API at `https://api.tripsy.app`. Public API paths do not include an `/api` prefix.

## Operating Rules

- Prefer friendly commands over `tripsy request` when a wrapper exists.
- Use `--json` for agent-readable output. Use `--quiet` only when raw `data` is needed without the envelope.
- Read the `ok`, `summary`, `data`, and `breadcrumbs` fields from JSON envelopes.
- Follow `breadcrumbs` when navigating related resources.
- Do not print stored tokens unless the user explicitly asks for token output.
- Do not ask the user for passwords in chat. Ask them to run `tripsy auth login --username USERNAME` locally, or use `TRIPSY_TOKEN`.
- Use exact ISO-8601 UTC datetimes for timed items, for example `2026-06-03T09:00:00Z`.
- For trip dates, use date strings such as `2026-06-01`.
- For destructive commands, state what will be deleted before running the command when the user has not already been explicit.

## Discovery

Start each unfamiliar workflow with structured command discovery:

```sh
tripsy commands --json
tripsy trips --help --agent --json
tripsy activities --help --agent --json
```

Check the installed version:

```sh
tripsy --version
tripsy version --json
```

Run health checks:

```sh
tripsy doctor --json
tripsy doctor --verbose --json
```

## Authentication

Check auth before authenticated work:

```sh
tripsy auth status --json
```

If unauthenticated, ask the user to run one of:

```sh
tripsy auth login --username USERNAME
tripsy auth token set TOKEN
```

Environment overrides:

```sh
TRIPSY_TOKEN=...
TRIPSY_API_BASE=https://api.tripsy.app
TRIPSY_CONFIG_DIR=/custom/config
```

Logout:

```sh
tripsy auth logout --json
tripsy auth logout --local --json
```

## Output Handling

Most JSON output has this shape:

```json
{
  "ok": true,
  "data": {},
  "summary": "Current user",
  "breadcrumbs": [
    {
      "action": "show",
      "cmd": "tripsy trips show <id>"
    }
  ]
}
```

For list endpoints, `data.results` usually contains items. `tripsy trips list` also uses `data.results`, but the Tripsy API does not include `count` for that endpoint.

For detail commands in human output, the CLI displays all fields returned by the API. For agents, prefer `--json` and inspect `data` directly.

## Common Flags

Global flags:

```sh
--json
--quiet
--api-base URL
--token TOKEN
--config-dir DIR
```

List filters where supported:

```sh
--fields id,name,starts_at
--fields-exclude documents,emails
--updated-since 2026-03-17T00:00:00Z
--deleted
```

Mutation payload options:

```sh
--data '{"name":"Italy","timezone":"Europe/Rome"}'
--set key=value
--name Italy
--starts-at 2026-06-01
```

Field flags use kebab-case and map to API snake_case, for example `--starts-at` maps to `starts_at`.

## Account

Show current profile:

```sh
tripsy me show --json
```

Update profile:

```sh
tripsy me update --name "Updated Name" --timezone America/Sao_Paulo --default-currency USD --json
```

Useful profile fields include `name`, `username`, `email`, `language`, `timezone`, `default_currency`, `store_currency`, calendar preferences, notification preferences, and `photo_url`.

## Trips

List trips:

```sh
tripsy trips list --json
tripsy trips list --fields id,name,starts_at,ends_at,timezone --json
```

Show full trip details:

```sh
tripsy trips show TRIP_ID --json
```

Create a trip:

```sh
tripsy trips create --name "Italy" --starts-at 2026-06-01 --ends-at 2026-06-15 --timezone Europe/Rome --json
```

Update a trip:

```sh
tripsy trips update TRIP_ID --description "Summer vacation" --json
tripsy trips update TRIP_ID --set cover_gradient=3 --json
```

Delete a trip:

```sh
tripsy trips delete TRIP_ID --json
```

Common trip fields: `name`, `timezone`, `hidden`, `description`, `starts_at`, `ends_at`, `cover_gradient`, `cover_image_url`, `has_dates`, `number_of_days`, and `guest_invites`.

## Itinerary Resources

All trip subresources require `--trip TRIP_ID`.

Activities:

```sh
tripsy activities list --trip TRIP_ID --json
tripsy activities list --trip TRIP_ID --activity-type sightseeing --json
tripsy activities show --trip TRIP_ID ACTIVITY_ID --json
tripsy activities create --trip TRIP_ID --name "Colosseum Tour" --activity-type sightseeing --starts-at 2026-06-03T09:00:00Z --ends-at 2026-06-03T11:00:00Z --timezone Europe/Rome --json
tripsy activities update --trip TRIP_ID ACTIVITY_ID --notes "Bring tickets" --checked true --json
tripsy activities delete --trip TRIP_ID ACTIVITY_ID --json
```

Useful activity fields: `activity_type`, `period`, `starts_at`, `ends_at`, `all_day`, `name`, `description`, `phone`, `website`, `checked`, `address`, `longitude`, `latitude`, `notes`, `timezone`, `price`, `currency`, and `assigned_users`.

Hostings:

```sh
tripsy hostings list --trip TRIP_ID --json
tripsy hostings show --trip TRIP_ID HOSTING_ID --json
tripsy hostings create --trip TRIP_ID --name "Hotel Eden" --starts-at 2026-06-01T14:00:00Z --ends-at 2026-06-05T11:00:00Z --timezone Europe/Rome --address "Via Ludovisi 49, Rome, Italy" --json
tripsy hostings update --trip TRIP_ID HOSTING_ID --room-number 402 --json
```

Useful hosting fields: `starts_at`, `ends_at`, `timezone`, `name`, `description`, `address`, `longitude`, `latitude`, `phone`, `room_type`, `room_number`, `website`, `notes`, `provider_reservation_code`, `price`, `currency`, and `assigned_users`.

Transportations:

```sh
tripsy transportations list --trip TRIP_ID --json
tripsy transportations list --trip TRIP_ID --transportation-type airplane --json
tripsy transportations show --trip TRIP_ID TRANSPORTATION_ID --json
tripsy transportations create --trip TRIP_ID --name "Flight to Rome" --transportation-type airplane --departure-description JFK --departure-at 2026-05-31T22:30:00Z --departure-timezone America/New_York --arrival-description FCO --arrival-at 2026-06-01T10:30:00Z --arrival-timezone Europe/Rome --json
```

Useful transportation fields: `transportation_type`, `departure_description`, `departure_at`, `departure_timezone`, `departure_address`, `departure_longitude`, `departure_latitude`, `arrival_description`, `arrival_at`, `arrival_timezone`, `arrival_address`, `arrival_longitude`, `arrival_latitude`, `company`, `seat_number`, `seat_class`, `transport_number`, `terminal`, `gate`, `price`, `currency`, and `assigned_users`.

Expenses:

```sh
tripsy expenses list --trip TRIP_ID --json
tripsy expenses show --trip TRIP_ID EXPENSE_ID --json
tripsy expenses create --trip TRIP_ID --title Dinner --price 78.5 --currency EUR --date 2026-06-03T20:00:00Z --json
tripsy expenses update --trip TRIP_ID EXPENSE_ID --price 82 --json
```

Expense fields: `title`, `date`, `price`, and `currency`.

Move an itinerary item to another trip:

```sh
tripsy activities update --trip OLD_TRIP_ID ACTIVITY_ID --update-trip NEW_TRIP_ID --json
tripsy hostings update --trip OLD_TRIP_ID HOSTING_ID --update-trip NEW_TRIP_ID --json
tripsy transportations update --trip OLD_TRIP_ID TRANSPORTATION_ID --update-trip NEW_TRIP_ID --json
```

## Collaborators

List collaborators and pending invitations:

```sh
tripsy collaborators --trip TRIP_ID --json
```

Inspect `permissions` in the returned data before assuming a user can edit expenses or documents.

## Email Addresses

List alternative emails:

```sh
tripsy emails list --json
```

Add an alternative email:

```sh
tripsy emails add work@example.com --json
```

Delete an alternative email:

```sh
tripsy emails delete EMAIL_ID --json
```

## Automation Inbox

List unprocessed automation emails:

```sh
tripsy inbox list --json
```

Show an inbox email:

```sh
tripsy inbox show EMAIL_ID --json
```

Rename or move an email:

```sh
tripsy inbox update EMAIL_ID --subject "Renamed itinerary email" --json
tripsy inbox update EMAIL_ID --trip-id TRIP_ID --json
tripsy inbox update EMAIL_ID --activity-id ACTIVITY_ID --json
tripsy inbox update EMAIL_ID --hosting-id HOSTING_ID --json
tripsy inbox update EMAIL_ID --transportation-id TRANSPORTATION_ID --json
```

Only one move target is applied. API priority is trip, activity, hosting, then transportation.

## Documents

Get a temporary document download URL:

```sh
tripsy documents get DOCUMENT_ID --json
```

Attach an external link to a trip:

```sh
tripsy documents attach --trip TRIP_ID --url https://example.com/reservation --title "Reservation" --json
```

Attach an external link to a subresource:

```sh
tripsy documents attach --trip TRIP_ID --parent activity:ACTIVITY_ID --url https://example.com/ticket --title "Ticket" --json
tripsy documents attach --trip TRIP_ID --parent hosting:HOSTING_ID --url https://example.com/hotel --title "Hotel" --json
tripsy documents attach --trip TRIP_ID --parent transportation:TRANSPORTATION_ID --url https://example.com/boarding-pass --title "Boarding pass" --json
```

Upload a private file and attach it:

```sh
tripsy documents upload ./boarding-pass.pdf --trip TRIP_ID --parent transportation:TRANSPORTATION_ID --title "Boarding pass" --json
```

Move or rename a document:

```sh
tripsy documents update DOCUMENT_ID --title "Updated ticket" --json
tripsy documents update DOCUMENT_ID --activity-id ACTIVITY_ID --json
tripsy documents update DOCUMENT_ID --trip-id TRIP_ID --json
```

Delete a document attachment:

```sh
tripsy documents delete --trip TRIP_ID DOCUMENT_ID --json
tripsy documents delete --trip TRIP_ID --parent activity:ACTIVITY_ID DOCUMENT_ID --json
```

## Upload URLs

Create a raw signed upload URL:

```sh
tripsy uploads create --purpose profile_photo --filename avatar.jpg --content-type image/jpeg --json
tripsy uploads create --purpose trip_cover --filename cover.jpg --content-type image/jpeg --json
tripsy uploads create --purpose document --parent-type trip --parent-id TRIP_ID --filename file.pdf --content-type application/pdf --json
```

Prefer `tripsy documents upload` for document files because it performs both the S3 upload and Tripsy attachment.

## Raw Requests

Use raw requests only when the friendly command surface does not cover an API route or flag yet.

```sh
tripsy request GET /v1/me --json
tripsy request GET /v1/trips --query fields=id,name --json
tripsy request PATCH /v1/me --data '{"timezone":"Europe/Rome"}' --json
```

For request bodies, use `--data` with a JSON object or repeated `--set key=value`.

## Installation

Install with the release script:

```sh
curl -fsSL https://raw.githubusercontent.com/tripsyapp/cli/main/scripts/install.sh | bash
```

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/tripsyapp/cli/main/scripts/install.sh | TRIPSY_VERSION=1.2.3 bash
```

Install from source with Go:

```sh
go install github.com/tripsyapp/cli/cmd/tripsy@latest
```

Verify:

```sh
tripsy --version
tripsy doctor --json
```

## Error Handling

- `401` usually means missing/invalid auth or failed public auth flow.
- `403` means authenticated but not allowed to edit or view the target.
- `404` means not found or not owned/accessible by the current user.
- `400` means validation failure; inspect the JSON error body.
- If a command has no friendly wrapper, use `tripsy request`.
