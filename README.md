# Tripsy CLI

`tripsy` is a command-line client for the public Tripsy API at `https://api.tripsy.app`.

The CLI follows the same practical shape as Basecamp CLI:

- usable human output in a terminal
- JSON envelopes when piped or when `--json` is passed
- breadcrumbs that suggest useful next commands
- a command catalog for agents through `tripsy commands --json` and `tripsy <command> --help --agent`
- local token storage under `~/.config/tripsy-cli`

## Build

Install the latest published version with Go:

```sh
go install github.com/tripsyapp/tripsy-cli/cmd/tripsy@latest
```

Or build from a checkout:

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

Interactive password prompts hide typed input on terminals. For automation, pass a token through `TRIPSY_TOKEN` or `tripsy auth token set`.

Or configure an existing token:

```sh
tripsy auth token set YOUR_TOKEN
```

Credentials are stored at:

```text
~/.config/tripsy-cli/credentials.json
```

Environment overrides:

```sh
TRIPSY_TOKEN=...
TRIPSY_API_BASE=https://api.tripsy.app
TRIPSY_CONFIG_DIR=/custom/config/dir
```

## Examples

```sh
tripsy me show
tripsy trips list
tripsy trips create --name Italy --starts-at 2026-06-01 --ends-at 2026-06-15 --timezone Europe/Rome
tripsy activities list --trip 42
tripsy activities create --trip 42 --name "Colosseum Tour" --activity-type sightseeing --starts-at 2026-06-03T09:00:00Z
tripsy transportations create --trip 42 --name "Flight to Rome" --transportation-type airplane --departure-description JFK --arrival-description FCO
tripsy expenses create --trip 42 --title Dinner --price 78.5 --currency EUR --date 2026-06-03T20:00:00Z
tripsy documents upload boarding-pass.pdf --trip 42 --parent transportation:303
tripsy request GET /v1/me --json
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
github.com/tripsyapp/tripsy-cli
```

If the GitHub repository path changes, update `go.mod` and the `go install` command above before tagging a release.
