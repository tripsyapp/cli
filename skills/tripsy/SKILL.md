# Tripsy CLI Agent Skill

Use this skill when an agent needs to inspect or modify Tripsy data through the local `tripsy` CLI.

## Command Discovery

Start with structured command discovery:

```sh
tripsy commands --json
tripsy trips --help --agent
```

All JSON command output uses an envelope:

```json
{
  "ok": true,
  "data": {},
  "summary": "",
  "breadcrumbs": []
}
```

Use `--quiet` when only raw API data is wanted.

## Authentication

Check auth before making API calls:

```sh
tripsy auth status --json
```

If no token is configured, ask the user to run one of:

```sh
tripsy auth login --username USERNAME
tripsy auth token set TOKEN
```

Do not print stored tokens unless the user explicitly asks.

## Common Workflows

List trips:

```sh
tripsy trips list --json
```

Inspect a trip and its itinerary:

```sh
tripsy trips show TRIP_ID --json
tripsy activities list --trip TRIP_ID --json
tripsy hostings list --trip TRIP_ID --json
tripsy transportations list --trip TRIP_ID --json
tripsy expenses list --trip TRIP_ID --json
```

Create a trip:

```sh
tripsy trips create --name "Italy" --starts-at 2026-06-01 --ends-at 2026-06-15 --timezone Europe/Rome --json
```

Attach a link document:

```sh
tripsy documents attach --trip TRIP_ID --url https://example.com --title "Reservation" --json
```

Upload a private document:

```sh
tripsy documents upload ./boarding-pass.pdf --trip TRIP_ID --parent transportation:TRANSPORTATION_ID --json
```

Fallback to raw API requests for missing wrappers:

```sh
tripsy request GET /v1/me --json
tripsy request PATCH /v1/me --data '{"timezone":"Europe/Rome"}' --json
```

## Notes

- Public API URLs do not include an `/api` prefix.
- Authenticated requests use `Authorization: Token ...`.
- Most list commands support `--fields`, `--fields-exclude`, `--updated-since`, and `--deleted` when the public API supports them.
- Trip subresource commands require `--trip` because public routes are scoped under `/v1/trip/{trip_id}`.
