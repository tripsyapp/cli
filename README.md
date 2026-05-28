# Tripsy CLI

`tripsy` is a command-line client for the public Tripsy API at `https://api.tripsy.app`. The project also ships `tripsy-mcp`, a Model Context Protocol server that exposes typed Tripsy tools for agents and MCP-capable apps.

API documentation is available at [docs.api.tripsy.app](https://docs.api.tripsy.app).

## What You Get

- Human-readable terminal output by default.
- JSON envelopes when output is piped or `--json` is passed.
- Raw JSON data with `--quiet`.
- Breadcrumbs that suggest useful next commands.
- A command catalog for humans and agents through `tripsy commands --json`.
- Secure token storage using the OS credential store when available, with explicit file fallback for automation.
- A local or hosted MCP server for structured agent workflows.

Agent-specific itinerary guidance does not belong in this README. Keep it in [AGENTS.md](./AGENTS.md) for repo-level agent instructions and [skills/tripsy/SKILL.md](./skills/tripsy/SKILL.md) for installable Codex skill guidance.

## Install

Install the latest GitHub release:

```sh
curl -fsSL https://tripsy.app/install_cli | bash
```

The installer downloads the latest release, verifies checksums, installs `tripsy` and `tripsy-mcp` into `~/.local/bin`, and adds that directory to your shell PATH when needed.

Install with Go:

```sh
go install github.com/tripsyapp/cli/cmd/tripsy@latest
go install github.com/tripsyapp/cli/cmd/tripsy-mcp@latest
```

Install a specific release:

```sh
curl -fsSL https://tripsy.app/install_cli | TRIPSY_VERSION=1.2.3 bash
```

Install into a custom directory:

```sh
curl -fsSL https://tripsy.app/install_cli | TRIPSY_BIN_DIR=/usr/local/bin bash
```

Build from a checkout:

```sh
make build
bin/tripsy --help
bin/tripsy-mcp --version
```

## Authenticate

Login with Tripsy credentials:

```sh
tripsy auth login --username you@example.com
```

Interactive password prompts hide typed input on terminals. Tokens are stored in the OS credential store when available. On macOS, Tripsy uses Keychain by default.

For automation, pass a token through `TRIPSY_TOKEN` or store one explicitly:

```sh
tripsy auth token set YOUR_TOKEN
```

Non-secret CLI config is stored at:

```text
~/.config/tripsy-cli/credentials.json
```

Environment overrides:

```sh
TRIPSY_TOKEN=...
TRIPSY_API_BASE=https://api.tripsy.app
TRIPSY_CONFIG_DIR=/custom/config/dir
TRIPSY_AUTH_BACKEND=auto|keychain|file
```

Use `TRIPSY_AUTH_BACKEND=file` only when you need file token storage for headless automation or compatibility.

## CLI Tools

Run `tripsy commands` for the current command catalog, or `tripsy commands --json` for agent-readable metadata.

| Command | Purpose | Subcommands |
| --- | --- | --- |
| `tripsy auth` | Authenticate and manage the stored API token. | `login`, `logout`, `status`, `token`, `reset-password`, `change-password` |
| `tripsy me` | Read or update the current Tripsy profile. | `show`, `update` |
| `tripsy trips` | List, create, inspect, update, and soft-delete trips. | `list`, `following`, `show`, `create`, `update`, `delete` |
| `tripsy hostings` | Manage hotel and lodging plans scoped to a trip. | `list`, `show`, `create`, `update`, `delete` |
| `tripsy activities` | Manage scheduled or unscheduled trip activities. | `list`, `show`, `create`, `update`, `delete` |
| `tripsy transportations` | Manage flights, trains, cars, and other transport. | `list`, `show`, `create`, `update`, `delete` |
| `tripsy expenses` | Manage trip expenses. | `list`, `show`, `create`, `update`, `delete` |
| `tripsy categories` | Manage custom activity categories. | `list`, `show`, `create`, `update`, `replace`, `delete` |
| `tripsy collaborators` | List collaborators and pending invitations for a trip. | `list` |
| `tripsy emails` | Manage alternative email addresses. | `list`, `add`, `delete` |
| `tripsy inbox` | Review automation emails that still need manual handling. | `list`, `show`, `update`, `delete` |
| `tripsy documents` | Get download URLs, move documents, attach links, upload files, and delete documents. | `get`, `update`, `attach`, `upload`, `delete` |
| `tripsy uploads` | Create raw backend-signed S3 upload URLs. | `create` |
| `tripsy request` | Make a raw Tripsy API request for routes without a friendly command. | none |
| `tripsy commands` | Print the CLI command catalog. | none |
| `tripsy doctor` | Check config, token presence, and authenticated API access. | none |
| `tripsy version` | Print the Tripsy CLI version. | none |

Common examples:

```sh
tripsy me show
tripsy trips list
tripsy trips following
tripsy trips create --name Italy --starts-at 2026-06-01 --ends-at 2026-06-15 --timezone Europe/Rome --cover-image-url "https://images.unsplash.com/photo-1529260830199-42c24126f198?ixlib=rb-4.1.0"
tripsy activities list --trip 42
tripsy activities create --trip 42 --name "Colosseum Tour" --activity-type tour --starts-at 2026-06-03T09:00:00Z --ends-at 2026-06-03T11:00:00Z --timezone Europe/Rome --address "Piazza del Colosseo, 1, 00184 Rome, Italy" --latitude 41.8902 --longitude 12.4922
tripsy hostings create --trip 42 --name "Hotel Eden" --starts-at 2026-06-01T14:00:00Z --ends-at 2026-06-05T11:00:00Z --timezone Europe/Rome
tripsy transportations create --trip 42 --name "Flight to Rome" --transportation-type airplane --departure-description JFK --arrival-description FCO
tripsy expenses create --trip 42 --title Dinner --price 78.5 --currency EUR --date 2026-06-03T20:00:00Z
tripsy categories create --name "Golf Clubs" --slug golf-clubs --icon-name golf --color AABBCC
tripsy documents upload boarding-pass.pdf --trip 42 --parent transportation:303
tripsy request GET /v1/me --json
tripsy doctor --verbose
```

Most trip subresource commands require `--trip <trip-id>` because the public API scopes those resources under a trip.

## Output

When output is piped, or when `--json` is passed, commands emit an envelope:

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

Use `--quiet` to print raw JSON data only.

## MCP Server

Use Tripsy's MCP server when an agent or app supports MCP. Prefer MCP for model-driven workflows because tools expose names, descriptions, schemas, structured results, and safety annotations.

### Hosted Endpoint

Tripsy operates a public MCP server at `https://mcp.tripsy.app`. OAuth-capable clients such as Claude and ChatGPT can connect directly without installing anything. Authentication uses the Tripsy OAuth authorization flow at `https://my.tripsy.app`.

The previous `https://mcp.tripsy.app/mcp` endpoint remains available for existing clients.

### Self-Hosted Stdio

Run `tripsy-mcp` locally when you want full control or stdio transport. It uses the same Tripsy token, config directory, API base URL, and secure token storage as the CLI.

```sh
tripsy-mcp
```

Example MCP client configuration:

```json
{
  "mcpServers": {
    "tripsy": {
      "command": "tripsy-mcp"
    }
  }
}
```

### Self-Hosted HTTP

Run a local streamable HTTP server:

```sh
tripsy-mcp --transport http --http-addr 127.0.0.1:8787 --http-path /mcp
```

The default HTTP endpoint path is `/mcp`, so this is equivalent:

```sh
tripsy-mcp --transport http --http-addr 127.0.0.1:8787
```

To host a remote MCP endpoint, run the HTTP server behind TLS:

```sh
tripsy-mcp --transport http --http-addr 127.0.0.1:8787 --http-path /mcp --disable-raw-request
```

Then proxy public paths to the local MCP server:

```text
https://mcp.tripsy.app/ -> http://127.0.0.1:8787/
https://mcp.tripsy.app/mcp -> http://127.0.0.1:8787/mcp
```

HTTP MCP always requires each request to include `Authorization: Bearer <Tripsy token>`. The server validates that token against `/v1/me` and uses it only for that downstream Tripsy API request, so each remote client acts as its own Tripsy user.

HTTP mode intentionally ignores `--token`, `TRIPSY_TOKEN`, keychain tokens, and legacy `credentials.json` tokens to avoid server-side credential fallback. For public hosted servers, keep `--disable-raw-request` enabled unless you intentionally want to expose the broader `tripsy_raw_request` tool.

For OAuth-capable remote clients, configure the public MCP URL and Tripsy OAuth issuer:

```sh
TRIPSY_MCP_PUBLIC_URL=https://mcp.tripsy.app
TRIPSY_OAUTH_ISSUER=https://my.tripsy.app
TRIPSY_OAUTH_SCOPES="profile email"
TRIPSY_API_BASE=https://api.tripsy.app
```

With those values, unauthenticated requests to `/` and `/mcp` include a `WWW-Authenticate` challenge pointing at `https://mcp.tripsy.app/.well-known/oauth-protected-resource`. That metadata advertises `https://my.tripsy.app` as the OAuth authorization server and validates OAuth bearer access tokens through `https://my.tripsy.app/oauth/userinfo`.

## MCP Tools

The MCP server exposes the tools below. All tools are closed-world: they only interact with the Tripsy API for the authenticated Tripsy account, not arbitrary external services or URLs.

### MCP Tool Conventions

- Timed fields are always UTC ISO-8601 timestamps, for example `2026-06-03T09:00:00Z`. Pair them with the relevant local IANA timezone field (`timezone`, `departure_timezone`, or `arrival_timezone`).
- When displaying activity or lodging dates/times, MCP clients must convert UTC `starts_at` and `ends_at` values into that item's `timezone` before formatting local date/time.
- When displaying transportation dates/times, MCP clients must convert UTC `departure_at` with `departure_timezone` and UTC `arrival_at` with `arrival_timezone`; do not apply one endpoint's timezone to the other endpoint unless the fields explicitly match.
- Trip date fields use calendar dates such as `2026-06-01`; itinerary item times use UTC timestamps.
- Activity creation requires numeric `latitude` and `longitude`; `tripsy_activities_create` rejects activity payloads without both coordinates so the Tripsy map is populated.
- Activity `activity_type` values may be built-in category slugs or visible custom category slugs. Custom category slugs are only valid on Activity objects through `activity_type`; they are not lodging, transportation, expense, or trip categories. When an MCP client sees an activity type that is not one of the documented built-in activity categories, it must fetch visible custom categories through `tripsy_categories_list` and resolve the slug there before displaying the activity category name, icon, or color.
- Activity, hosting, and transportation objects use `provider_reservation_code` for the provider-issued reservation, confirmation, or booking code for that item. Transportation objects use `transport_number` separately for the flight, train, bus, or service number.
- For trip covers, save only a real direct Unsplash CDN `cover_image_url` copied from an image result. The MCP server validates direct Unsplash URL shape. If the client also has external URL access, confirm the image URL is reachable and not returning a `404` before creating or updating the trip.
- Delete tools can be executed when requested. Tripsy delete operations are recoverable, so they can be undone if necessary; use list filters such as `deleted` when you need to inspect deleted records.

Read-only tools:

- `tripsy_status`: inspect MCP configuration and authentication state without revealing the token.
- `tripsy_itinerary_guidance`: return concise itinerary-building guidance for agents.
- `tripsy_me_show`: return the authenticated Tripsy profile.
- `tripsy_trips_list`: list trips where the authenticated user is travelling.
- `tripsy_trips_following_list`: list trips the authenticated user follows but is not travelling on.
- `tripsy_trips_show`: fetch one trip by id.
- `tripsy_activities_list`: list activities for a trip.
- `tripsy_activities_show`: fetch one activity by id.
- `tripsy_hostings_list`: list hostings for a trip.
- `tripsy_hostings_show`: fetch one hosting by id.
- `tripsy_transportations_list`: list transportations for a trip.
- `tripsy_transportations_show`: fetch one transportation by id.
- `tripsy_expenses_list`: list expenses for a trip.
- `tripsy_expenses_show`: fetch one expense by id.
- `tripsy_categories_list`: list visible custom activity categories.
- `tripsy_categories_show`: fetch one visible custom activity category by id.
- `tripsy_collaborators_list`: list collaborators and pending invitations for a trip.

Write tools:

- `tripsy_me_update`: update current profile fields.
- `tripsy_trips_create`: create a trip.
- `tripsy_trips_update`: update a trip.
- `tripsy_activities_create`: create an activity.
- `tripsy_activities_update`: update an activity.
- `tripsy_hostings_create`: create a hosting.
- `tripsy_hostings_update`: update a hosting.
- `tripsy_transportations_create`: create a transportation.
- `tripsy_transportations_update`: update a transportation.
- `tripsy_expenses_create`: create an expense.
- `tripsy_expenses_update`: update an expense.
- `tripsy_categories_create`: create a custom activity category.
- `tripsy_categories_update`: update a custom activity category.

Destructive tools:

- `tripsy_trips_delete`: soft-delete a trip.
- `tripsy_activities_delete`: delete an activity.
- `tripsy_hostings_delete`: delete a hosting.
- `tripsy_transportations_delete`: delete a transportation.
- `tripsy_expenses_delete`: delete an expense.
- `tripsy_categories_delete`: delete a custom activity category.
- `tripsy_raw_request`: make a raw request to supported Tripsy public API endpoints. This tool is disabled when the MCP server runs with `--disable-raw-request`.

Delete operations are available through MCP and may be used when the user asks to remove data. They are recoverable if they need to be undone later.

Trip list results are split by current-user travelling status: `tripsy_trips_list` returns trips where the authenticated user is travelling, and `tripsy_trips_following_list` returns trips the user follows but is not travelling on. For date handling, `has_dates` is authoritative: when `has_dates` is `false`, ignore `starts_at` and `ends_at` even if those fields are present.

## Development

```sh
make fmt
make check
make vulncheck
```

`make fmt` applies the pinned Go formatter (`gofumpt`) across the Go source tree. `make fmt-check` verifies formatting without modifying files. CI runs formatting checks, `go vet`, tests, and `govulncheck`.

## Publishing

This module is published as:

```text
github.com/tripsyapp/cli
```

If the GitHub repository path changes, update `go.mod` and the `go install` command above before tagging a release.

The install script expects GitHub release assets named like:

```text
tripsy_1.2.3_darwin_arm64.tar.gz
tripsy_1.2.3_linux_amd64.tar.gz
tripsy_1.2.3_windows_amd64.zip
checksums.txt
```

Each platform archive contains `tripsy`, `tripsy-mcp`, `README.md`, and `LICENSE`. The release workflow creates these assets when a `vX.Y.Z` tag is pushed.
