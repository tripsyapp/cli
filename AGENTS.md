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
- Create one item per actual stop, reservation, meal, tour, or activity. Do not bundle multiple places into one activity.
- Use start and end times when possible. Send timed values as UTC ISO-8601 strings and set the local `timezone`.
- Add `address`, `latitude`, and `longitude` for location-based activities and lodging so the Tripsy map is populated.
- Use `hostings` for hotels/lodging. The lodging category slug is `lodging`.
- Use `transportations` for point-to-point movement and the transportation slugs listed below.
- For flights, create a transportation with `transportation_type` set to `airplane`, set `departure_description` and `arrival_description` to the airport IATA codes, include each airport's latitude and longitude, and omit `name` unless the user provided one.
- For transfer activities, create a transportation with `transportation_type` set to `roadtrip`, and fill both departure and arrival locations with name/description, address, latitude, and longitude.

## Avoid

- Do not use `unsplash.com/photos/...` as `cover_image_url`.
- Do not invent or transform Unsplash photo IDs into `images.unsplash.com` URLs; copy the real numeric photo asset URL.
- Do not create one activity named "Day 1 itinerary" or similar that contains multiple stops.
- Do not put hotels or lodging into activities.
- Do not put transfers into activities.
- Do not omit coordinates when a location is known.
- Do not use unsupported `activity_type` values such as `sightseeing`.

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

## Activity Categories

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
