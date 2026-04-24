# Tripsy CLI

`tripsy` is a command-line client for the public Tripsy API at `https://api.tripsy.app`.

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

This installs the latest GitHub release into `~/.local/bin`, verifies the release checksum, and adds that directory to your shell PATH when needed.

## Other Installation Methods

Install the latest published version with Go:

```sh
go install github.com/tripsyapp/cli/cmd/tripsy@latest
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

## Examples

```sh
tripsy me show
tripsy trips list
tripsy trips create --name Italy --starts-at 2026-06-01 --ends-at 2026-06-15 --timezone Europe/Rome
tripsy activities list --trip 42
tripsy activities create --trip 42 --name "Colosseum Tour" --activity-type tour --starts-at 2026-06-03T09:00:00Z --ends-at 2026-06-03T11:00:00Z --timezone Europe/Rome --latitude 41.8902 --longitude 12.4922
tripsy transportations create --trip 42 --name "Flight to Rome" --transportation-type airplane --departure-description JFK --arrival-description FCO
tripsy expenses create --trip 42 --title Dinner --price 78.5 --currency EUR --date 2026-06-03T20:00:00Z
tripsy documents upload boarding-pass.pdf --trip 42 --parent transportation:303
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
- Choose the most specific supported category slug for every activity.

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
- email and automation inbox: `emails`, `inbox`
- documents and uploads: `documents`, `uploads`

Use `tripsy request METHOD PATH` for any exposed API route that does not yet have a tailored command.

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

The release workflow creates these assets when a `vX.Y.Z` tag is pushed.
