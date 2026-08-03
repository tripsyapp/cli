# Agent Guidance

Use this file when an agent is creating or maintaining Tripsy itinerary data through the local `tripsy` CLI or `tripsy-mcp` server.

## CLI vs MCP

- Prefer `tripsy-mcp` when the client supports MCP. It exposes typed tools, structured results, safety annotations, and the same auth/config as the CLI.
- Use `tripsy` for direct terminal workflows, scripts, or when the current client cannot connect to MCP servers.
- MCP tool names use the `tripsy_<resource>_<action>` shape, for example `tripsy_itinerary_guidance`, `tripsy_trips_create`, `tripsy_activities_create`, and `tripsy_collaborators_list`.
- Use `tripsy_raw_request` only when no typed MCP tool covers a supported public Tripsy API route.

## Authentication

- Do not print stored tokens unless the user explicitly asks for token output.
- Prefer the default `TRIPSY_AUTH_BACKEND=auto`; it uses OS credential storage when available.
- Use `TRIPSY_AUTH_BACKEND=file` only for headless automation or compatibility.
- Non-secret config is stored in `credentials.json`; tokens should be in the secure backend whenever one is available.

## Itinerary Shape

- Set trip dates when planning a day-by-day itinerary.
- `trips list` returns trips where the authenticated user is travelling. Use `trips following` for trips the user follows but is not travelling on.
- `has_dates` is authoritative. If `has_dates` is `false`, ignore `starts_at` and `ends_at` even when those fields are present.
- Choose a destination-specific Unsplash image for leisure trips and set it as `cover_image_url`.
- Store a real direct Unsplash CDN URL copied from an image result, in the form `https://images.unsplash.com/photo-1562869929-bda0650edb1f?ixid=...&ixlib=rb-4.1.0`.
- The `images.unsplash.com` path must be `photo-<numeric timestamp>-<asset hash>`. Do not store the Unsplash page URL, and do not turn short photo IDs like `nWdsya5_Yms` into `https://images.unsplash.com/photo-nWdsya5_Yms`.
- Before saving a trip `cover_image_url`, validate that it is a real direct Unsplash CDN URL. If the client has external URL access, also confirm the image URL is reachable and not returning a `404`.
- Create one item per actual stop, reservation, meal, tour, or activity. Do not bundle multiple places into one activity.
- Use `provider_reservation_code` on activities, hostings, and transportations for the provider-issued reservation, confirmation, or booking code for that item.
- For transportation, keep `transport_number` for the flight, train, bus, or service number; use `provider_reservation_code` for the booking or confirmation code.
- Use start and end times when possible. Send all timed values as UTC ISO-8601 strings. Always set the local IANA `timezone` for the activity or lodging location, and `departure_timezone`/`arrival_timezone` for transportation endpoints.
- When displaying activity or lodging dates/times from MCP data, convert UTC `starts_at` and `ends_at` into the item's `timezone` before formatting local date/time.
- When displaying transportation dates/times from MCP data, convert UTC `departure_at` with `departure_timezone` and UTC `arrival_at` with `arrival_timezone`; do not apply one endpoint's timezone to the other endpoint unless the fields explicitly match.
- Activity creation through MCP requires `latitude` and `longitude`; include `address` when known so the Tripsy map is populated.
- Add `address`, `latitude`, and `longitude` for lodging when known so the Tripsy map is populated.
- Use `hostings` for hotels/lodging. The lodging category slug is `lodging`.
- Use `transportations` for point-to-point movement and the transportation slugs listed below.
- Activities can use either a documented built-in `activity_type` slug or a visible custom category slug. Custom category slugs are only valid on Activity objects through `activity_type`; do not use them for lodging, transportation, expenses, or trips. If an activity has an `activity_type` that is not in the built-in list, fetch visible custom categories through `tripsy_categories_list` or `tripsy categories list` and resolve the slug there before displaying the category name, icon, or color.
- For flights, create a transportation with `transportation_type` set to `airplane`, set `departure_description` and `arrival_description` to the airport IATA codes, include each airport's latitude and longitude, and omit `name` unless the user provided one.
- For transfer activities, create a transportation with `transportation_type` set to `roadtrip`, and fill both departure and arrival locations with name/description, address, latitude, and longitude.
- Delete operations can be executed when requested. Tripsy deletes are recoverable, so they can be undone if necessary.

## Avoid

- Do not use `unsplash.com/photos/...` as `cover_image_url`.
- Do not invent or transform Unsplash photo IDs into `images.unsplash.com` URLs; copy the real numeric photo asset URL.
- Do not save a malformed `cover_image_url`; check for `404` only when the client has external URL access.
- Do not create one activity named "Day 1 itinerary" or similar that contains multiple stops.
- Do not put hotels or lodging into activities.
- Do not put transfers into activities.
- Do not create activities without coordinates.
- Do not treat an unknown `activity_type` as invalid until you have checked whether it is a visible custom category. Do not invent ad hoc values such as `sightseeing`.

## Golden Path Example

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
    "longitude": 12.4882,
    "provider_reservation_code": "HTL-123456"
  },
  "activity": {
    "name": "Colosseum Tour",
    "activity_type": "tour",
    "starts_at": "2026-06-03T09:00:00Z",
    "ends_at": "2026-06-03T11:00:00Z",
    "timezone": "Europe/Rome",
    "address": "Piazza del Colosseo, 1, 00184 Rome, Italy",
    "latitude": 41.8902,
    "longitude": 12.4922,
    "provider_reservation_code": "TOUR-987654"
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
    "arrival_longitude": 12.4882,
    "provider_reservation_code": "CAR-246810"
  }
}
```

## Activity Categories

Activities normally use one of these built-in category slugs. Activities may also use custom category slugs returned by `tripsy_categories_list` or `tripsy categories list`. Custom category slugs are only for Activity objects through `activity_type`. MCP clients that render activities must handle both cases: display the built-in category metadata for known built-in slugs, and resolve custom slugs from visible custom categories so the correct custom category name is shown.

```text
concert, fit, general, kids, museum, note, relax, restaurant, shopping,
theater, tour, event, meeting, bar, cafe, parking, amusementPark, aquarium,
atm, bakery, bank, beach, brewery, campground, evCharger, fireStation,
fitnessCenter, foodMarket, gasStation, hospital, laundry, library, marina,
movieTheater, nationalPark, nightlife, park, pharmacy, police, postOffice,
publicTransport, restroom, school, stadium, university, winery, zoo
```

## Lodging Category

```text
lodging
```

## Transportation Categories

```text
airplane, bike, bus, car, roadtrip, cruise, ferry, motorcycle, train, walk
```

## Pull Request Conventions

- Do not prefix PR titles with `[codex]`.
- Do not add `codex` labels, tags, or title markers.
- Use a conventional PR title prefix that matches the change type:
  - `fix:` for bug fixes
  - `feat:` for user-facing features
  - `chore:` for maintenance, release, dependency, or tooling work
  - `test:` for test-only changes
  - `refactor:` for behavior-preserving code restructuring
  - `docs:` for documentation-only changes
- Add the matching GitHub label when opening or updating a PR:
  - `bug` for bug fixes
  - `feature` for user-facing features
  - `improvement` for behavior or UX improvements
  - `chore` for maintenance, release, dependency, or tooling work
  - `dependencies`, `github_actions`, `swift_package_manager`, or `performance` when those labels more specifically describe the work
- Include the issue identifier when available:
  - `fix(TRI-2234): correct ends in day count`
  - `feat(TRI-2201): add route preference picker`
  - `chore: update release tags`
- Open PRs as ready for review unless explicitly asked to open a draft PR.
- Every PR body must include `Summary`, `Implementation Details`, and `Validation` sections, plus the issue/ticket reference when available.
- The `Implementation Details` section must explain the material code changes at file and symbol level: list newly created files, types, or components, and describe the functions, models, views, or existing files that were changed and what each change accomplishes.
- Keep the `Implementation Details` section synchronized with the final diff before opening or updating the PR.
