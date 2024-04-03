# About

Package `ps2` provides types, constants, and functions for working with PlanetSide 2 APIs.

These modules do much of the heavy lifting in converting raw data sources to strongly-typed values.

This repository also contains sub-modules for working with various services.
Some 3rd-party sites have small inclusions as well;
these may be expanded upon in the future.

### Goals

The goal is to ease the burden of building packages that can work together by providing a standard library of building blocks.

Imagine being able to use modules with function signatures like this:

```go
func GetOutfitMembers(id ps2.OutfitID) ([]ps2.CharacterID, error) {
    //...
}
```

By adopting a standard base for parameter and return types,
packages can much more easily work together with all the benefits of type-safety.
No passing around strings and forgetting if `""` or `"0"` is the empty value.
No passing around generic int types and passing an instanced zone ID to a function that expected a zone id.

## ps2

Package `ps2` is the main, top-level package that contains basic types and functions that are the building blocks for all other packages.

`ps2` primarily contains ID types and named constants for the most commonly used IDs,
like `ps2.Amerish` or `ps2.Infiltrator`.
This package is generally not useful by itself.

## census

Package [`census`](./census/) contains structs and functions for working with the official Daybreak Games Census API for PlanetSide 2.

`census` builds on `ps2` by defining the structs returned by various census collections.
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

`census` also contains a number of functions for querying the census api. This includes a few helpful features by default:

-   Rate and concurrency limiting - your code can go func the census as often as you like without protection.

-   Automatic pagination for loading entire static collections.
-   Correct serialization/deserialization - is the event duration in milliseconds? Seconds? You don't care; it's a `time.Time`.

View the package docs for more information.

## wsc

Package [`wsc`](./event/wsc/) contains a **W**eb**S**ocket **C**lient for interacting with the PlanetSide 2 realtime event push service.

The JSON structure of the realtime event service can be surprisingly annoying to work with in Go.

`wsc` takes care of that.

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

The state manager keeps itself updated in realtime by attaching to a `wsc.Client` and listening for various population and territory events.

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
