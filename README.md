# Tripsy CLI

`tripsy` is a command-line client for the public Tripsy API at `https://api.tripsy.app`. The project also ships `tripsy-mcp`, a Model Context Protocol server that exposes typed tools for core Tripsy account and trip workflows.

The CLI follows the same practical shape as [Basecamp CLI](https://github.com/basecamp/basecamp-cli):

- usable human output in a terminal
- JSON envelopes when piped or when `--json` is passed
- breadcrumbs that suggest useful next commands
- a command catalog for agents through `tripsy commands --json` and `tripsy <command> --help --agent`
- secure token storage using the OS credential store when available, with explicit file fallback for automation

## Quick Start

```sh
curl -fsSL https://tripsy.app/install_cli | bash
```

This installs the latest GitHub release into `~/.local/bin`, verifies the release checksum, installs `tripsy` and `tripsy-mcp`, and adds that directory to your shell PATH when needed.

## Other Installation Methods

Install the latest published version with Go:

```sh
go install github.com/tripsyapp/cli/cmd/tripsy@latest
go install github.com/tripsyapp/cli/cmd/tripsy-mcp@latest
```

Install a specific release with the script:

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

For development:

```sh
make check
```

## Authentication

Login with Tripsy credentials:

```sh
tripsy auth login --username you@example.com
```

Interactive password prompts hide typed input on terminals. Tokens are stored in the OS credential store when available. On macOS, Tripsy uses Keychain by default.

For automation, pass a token through `TRIPSY_TOKEN` or `tripsy auth token set`.

Or configure an existing token:

```sh
tripsy auth token set YOUR_TOKEN
```

Non-secret CLI config is stored at:

```text
~/.config/tripsy-cli/credentials.json
```

For compatibility, file token storage is still available with:

```sh
TRIPSY_AUTH_BACKEND=file
```

Environment overrides:

```sh
TRIPSY_TOKEN=...
TRIPSY_API_BASE=https://api.tripsy.app
TRIPSY_CONFIG_DIR=/custom/config/dir
TRIPSY_AUTH_BACKEND=auto|keychain|file
```

## MCP Server

Use Tripsy's MCP server when an agent or app supports MCP. Two ways to connect:

### Hosted endpoint (recommended)

Tripsy operates a public MCP server at `https://mcp.tripsy.app/mcp`. OAuth-capable clients such as Claude and ChatGPT can connect directly without installing anything. Authentication uses the Tripsy OAuth authorization flow at `https://my.tripsy.app`.

Example MCP client configuration:

<img width="2778" height="2218" alt="CleanShot 2026-05-11 at 21 06 30@2x" src="https://github.com/user-attachments/assets/46f9aaf0-b6ed-4b17-9deb-ad3cf0aefd16" />

On first use, the client opens the Tripsy OAuth consent screen. After approval, the client stores the bearer token and reuses it for every request.

### Self-hosted

Run `tripsy-mcp` locally when you want full control or a stdio transport. It uses the same Tripsy token, config directory, API base URL, and secure token storage as the CLI.

Run the default stdio server:

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

Run a local streamable HTTP server instead:

```sh
tripsy-mcp --transport http --http-addr 127.0.0.1:8787 --http-path /mcp
```

The default HTTP endpoint path is `/mcp`, so this is equivalent to:

```sh
tripsy-mcp --transport http --http-addr 127.0.0.1:8787
```

To host your own remote MCP endpoint equivalent to `https://mcp.tripsy.app/mcp`, run the HTTP server behind TLS:

```sh
tripsy-mcp --transport http --http-addr 127.0.0.1:8787 --http-path /mcp --disable-raw-request
```

Then proxy the public path to the local MCP server:

```text
https://mcp.tripsy.app/mcp -> http://127.0.0.1:8787/mcp
```

HTTP MCP always requires each request to include `Authorization: Bearer <Tripsy token>`. The server validates that token against `/v1/me` and uses it only for that downstream Tripsy API request, so each remote client acts as its own Tripsy user. HTTP mode intentionally ignores `--token`, `TRIPSY_TOKEN`, keychain tokens, and legacy `credentials.json` tokens to avoid any server-side credential fallback.

For public hosted servers, keep `--disable-raw-request` enabled unless you intentionally want to expose the broad `tripsy_raw_request` tool. The typed tools cover the core Tripsy workflows with a narrower API surface.

When hosting the MCP server for OAuth-capable remote clients such as Claude or ChatGPT, configure the public MCP URL and Tripsy OAuth issuer so clients can discover the authorization flow:

```sh
TRIPSY_MCP_PUBLIC_URL=https://mcp.tripsy.app
TRIPSY_OAUTH_ISSUER=https://my.tripsy.app
TRIPSY_OAUTH_SCOPES="profile email"
TRIPSY_API_BASE=https://api.tripsy.app
```

With those values, unauthenticated requests to `/mcp` include a `WWW-Authenticate` challenge pointing at `https://mcp.tripsy.app/.well-known/oauth-protected-resource`. That metadata advertises `https://my.tripsy.app` as the OAuth authorization server and validates OAuth bearer access tokens through `https://my.tripsy.app/oauth/userinfo`.

The MCP server exposes typed tools such as `tripsy_itinerary_guidance`, `tripsy_trips_create`, `tripsy_activities_create`, `tripsy_hostings_create`, `tripsy_transportations_create`, `tripsy_expenses_create`, `tripsy_collaborators_list`, and `tripsy_raw_request`. Tool schemas and descriptions carry the same itinerary guidance as the CLI docs: choose a direct Unsplash `cover_image_url`, create one item per stop or reservation, set precise categories, and include coordinates for map-ready items.

Use the CLI when you want direct terminal commands, shell scripts, or human-readable output. Use MCP when a model client should discover Tripsy operations through structured tool schemas instead of composing shell commands and parsing CLI help.

## Examples

```sh
tripsy me show
tripsy trips list
tripsy trips create --name Italy --starts-at 2026-06-01 --ends-at 2026-06-15 --timezone Europe/Rome --cover-image-url "https://images.unsplash.com/photo-1529260830199-42c24126f198?ixlib=rb-4.1.0"
tripsy activities list --trip 42
tripsy activities create --trip 42 --name "Colosseum Tour" --activity-type tour --starts-at 2026-06-03T09:00:00Z --ends-at 2026-06-03T11:00:00Z --timezone Europe/Rome --address "Piazza del Colosseo, 1, 00184 Rome, Italy" --latitude 41.8902 --longitude 12.4922
tripsy transportations create --trip 42 --name "Flight to Rome" --transportation-type airplane --departure-description JFK --arrival-description FCO
tripsy expenses create --trip 42 --title Dinner --price 78.5 --currency EUR --date 2026-06-03T20:00:00Z
tripsy request GET /v1/me --json
```

## Agent Itinerary Rules

When building a Tripsy itinerary for a user or agent workflow:

- Set trip dates whenever the itinerary needs day-by-day planning. Use trip date strings such as `2026-06-01`.
- Choose a high-quality destination-specific Unsplash image for the trip cover when possible, and set it with `cover_image_url`.
- Store the direct `images.unsplash.com/photo-...?...&ixlib=rb-...` URL, not the Unsplash page URL. The app will add its own display parameters.
- Create one Tripsy item per actual stop, reservation, meal, tour, or activity. Do not combine a full day or multiple places into one activity.
- Use exact ISO-8601 UTC datetimes for timed items, plus the local `timezone`, for example `2026-06-03T09:00:00Z`.
- Set `latitude` and `longitude` for every location-based activity, hosting, and transportation endpoint so Tripsy's map is populated.
- Use `hostings` for hotels/lodging. The lodging category slug is `lodging`.
- Use `transportations` for flights, trains, cars, buses, cruises, ferries, roadtrips, walks, and similar point-to-point movement.
- For flights, create a transportation with `transportation_type` set to `airplane`, set `departure_description` and `arrival_description` to the airport IATA codes, include each airport's latitude and longitude, and omit `name` unless the user provided one.
- For transfer activities, create a transportation with `transportation_type` set to `roadtrip`, and fill both departure and arrival locations with name/description, address, latitude, and longitude.
- Choose the most specific supported category slug for every activity.

Avoid these common itinerary mistakes:

- Do not use `unsplash.com/photos/...` as `cover_image_url`.
- Do not create one activity named "Day 1 itinerary" or similar that contains multiple stops.
- Do not put hotels or lodging into activities.
- Do not put transfers into activities.
- Do not omit coordinates when a location is known.
- Do not use unsupported `activity_type` values such as `sightseeing`.

Golden path payload shape:

```json
{
  "trip": {
    "name": "Rome",
    "timezone": "Europe/Rome",
    "starts_at": "2026-06-01",
    "ends_at": "2026-06-05",
    "cover_image_url": "https://images.unsplash.com/photo-1529260830199-42c24126f198?ixlib=rb-4.1.0"
  },
  "hosting": {
    "name": "Hotel Eden",
    "starts_at": "2026-06-01T14:00:00Z",
    "ends_at": "2026-06-05T11:00:00Z",
    "timezone": "Europe/Rome",
    "address": "Via Ludovisi 49, 00187 Rome, Italy",
    "latitude": 41.9081,
    "longitude": 12.4882
  },
  "activity": {
    "name": "Colosseum Tour",
    "activity_type": "tour",
    "starts_at": "2026-06-03T09:00:00Z",
    "ends_at": "2026-06-03T11:00:00Z",
    "timezone": "Europe/Rome",
    "address": "Piazza del Colosseo, 1, 00184 Rome, Italy",
    "latitude": 41.8902,
    "longitude": 12.4922
  },
  "transfer": {
    "name": "Transfer to Hotel Eden",
    "transportation_type": "roadtrip",
    "departure_description": "Rome Fiumicino Airport",
    "departure_address": "Via dell'Aeroporto di Fiumicino, 00054 Fiumicino RM, Italy",
    "departure_latitude": 41.8003,
    "departure_longitude": 12.2389,
    "arrival_description": "Hotel Eden",
    "arrival_address": "Via Ludovisi 49, 00187 Rome, Italy",
    "arrival_latitude": 41.9081,
    "arrival_longitude": 12.4882
  }
}
```

Activity category slugs:

```text
concert, fit, general, kids, museum, note, relax, restaurant, shopping,
theater, tour, event, meeting, bar, cafe, parking, amusementPark, aquarium,
atm, bakery, bank, beach, brewery, campground, evCharger, fireStation,
fitnessCenter, foodMarket, gasStation, hospital, laundry, library, marina,
movieTheater, nationalPark, nightlife, park, pharmacy, police, postOffice,
publicTransport, restroom, school, stadium, university, winery, zoo
```

Transportation category slugs:

```text
airplane, bike, bus, car, roadtrip, cruise, ferry, motorcycle, train, walk
```

Lodging category slug:

```text
lodging
```

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

## Command Coverage

Friendly commands wrap the currently exposed public API:

- auth/account: `auth`, `me`
- trips: `trips`
- trip subresources: `hostings`, `activities`, `transportations`, `expenses`, `collaborators`

Use `tripsy request METHOD PATH` for any exposed API route that does not yet have a tailored command.

The MCP server currently covers account, trips, trip subresources, collaborators, and supported raw requests.

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
