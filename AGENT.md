# Agent Guidance

Use this file when an agent is creating or maintaining Tripsy itinerary data through the local `tripsy` CLI.

## Itinerary Shape

- Set trip dates when planning a day-by-day itinerary.
- Choose a destination-specific Unsplash image for leisure trips and set it as `cover_image_url`.
- Store the direct `images.unsplash.com/photo-...?...&ixlib=rb-...` URL. Do not store the Unsplash page URL.
- Create one item per actual stop, reservation, meal, tour, or activity. Do not bundle multiple places into one activity.
- Use start and end times when possible. Send timed values as UTC ISO-8601 strings and set the local `timezone`.
- Add `address`, `latitude`, and `longitude` for location-based activities and lodging so the Tripsy map is populated.
- Use `hostings` for hotels/lodging. The lodging category slug is `lodging`.
- Use `transportations` for point-to-point movement and the transportation slugs listed below.

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
