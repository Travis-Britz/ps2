# About

Package `ps2` provides modules for working with PlanetSide 2 APIs.

Some 3rd-party sites have small inclusions as well;
these may be expanded upon in the future.

### Goals

The goal is to provide a standard library of building blocks that PlanetSide packages can build upon.

## ps2

Package `ps2` is the main, top-level package that contains basic types and functions that are the building blocks for all other packages.

`ps2` primarily contains ID types and named constants for the most commonly used IDs,
like `ps2.Amerish` or `ps2.Infiltrator`.
This package is generally not useful by itself.

## census

Package [`census`](./census/) contains types and functions for working with the official Census API.

### Types

`census` builds on `ps2` by defining the structs returned by various census collections.

Serialization/deserialization - is the event duration in milliseconds? Seconds? You don't care; it's a `time.Time`.

These structs can be combined together to quickly build up responses for complicated census queries.

These types can also be embedded in more complex structs. For example, a struct for parsing results from `census.lithafalcon.cc`, which contains a superset of the fields returned by census for most collections, could be built like this:

```go
type Faction struct {
	census.Faction
	ShortName ps2.Localization `json:"short_name"`
	HudTint   int              `json:"hud_tint_rgb,string"`
}
```

These are trivial examples of course, but working with JSON from the Census API is surprisingly tedious in Go.
By using and passing around types from `ps2` and structs from `census`,
programs can work together much more easily,
and with all the advantages of type safety.

### HTTP Client

The `census` HTTP Client includes a number of features out of the box that every program should have:

-   Concurrency limits - go func the census API any way you like (respectfully).

    This package will automatically apply a limit of two concurrent requests.
    Additional requests will wait until active requests succeed or fail.

-   Rate limits.

    Similar to the concurrency limit (but different), this package will also apply a rate limit to census requests. Unlike the concurrency limit, the rate limit can be configured. The package includes a reasonable default.

    Even though requests are rate limited, programs should still cache static results whenever possible.

-   Error handling and retries.

    Census requests may fail for various reasons. Most failed requests can be retried immediately. This package will automatically retry up to a configurable number of times for known error types, which greatly simplifies most use cases. The default client retries once.

    Errors are also tracked, and if too many consecutive errors are encountered then additional requests will short circuit and immediately return an error for a period of time.

## wsc

Package [`wsc`](./event/wsc/) contains a **W**eb**S**ocket **C**lient for interacting with the PlanetSide 2 realtime event push service.

Every message received by the client is parsed into an [`event`](./event/) struct,
such as `event.Death`, `event.VehicleDestroy`, etc.
This makes working with the push service a breeze in Go.

In Progress:

-   message deduplication

TODO:

-   dropped connection detection

([Nanite Systems](https://nanite-systems.net/) should also be compatible as a drop-in replacement to handle the reliability of connections)

## state

Package `state` is for tracking live game state:

-   population (per world and zone)
-   continent lock status
-   territory control
-   alert status

The state manager keeps itself updated in real-time by attaching to a `wsc.Client` and listening for various population and territory events.

The state manager can also emit events when continents change state (including unlocks), when territory control changes during an alert, and when populations are counted (every 15 seconds).

## psmap

Package `psmap` contains various map-related functions:

-   Territory percentage calculation (including cut off regions)
-   Outlining useful for generating polygons for map regions
-   SVG rendering of map territory control

## pack2

todo (maybe)

### Compatibility Promise

The release of ps2 version 1, expected 2038-01-19 03:14:08Z, will be an insignificant milestone in the development of the package. Version 1 is an unstable platform for the growth of PlanetSide-related programs and projects depending on this package.

Although we expect the vast majority of programs to drift out of compatibility on their own over time, we hereby promise to release breaking changes with every commit to this repository.

These considerations apply to successive point releases. For instance, code that runs under ps2 1.0 should be incompatible with ps2 1.0.1. Semver be damned. #LiveFree in the NC.

Therefore, if you manage to find a version of this package that successfully compiles and works as expected in your program, it is advised to require that specific sha in your go.mod and never look back.

(/s ...I think.)
