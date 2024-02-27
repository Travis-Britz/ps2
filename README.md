# About

Package `ps2` provides types, constants, and functions for working with PlanetSide 2 APIs.

The goal is to provide a standard library for all PlanetSide 2 Go packages to use.

This repository also contains sub-modules for working with various services:

-   `census` contains structs and functions for working with the official Daybreak Games Census API for PlanetSide 2.
-   `ps2/event` contains structs for working with the realtime events API.
-   `ps2/event/wsc` contains a websocket client for connecting to a realtime events API.
-   `sanctuary` contains structs for interacting with the unofficial 3rd-party [Sanctuary.Census](https://github.com/PS2Sanctuary/Sanctuary.Census) project built by Falcon.

Some other 3rd-party sites have small inclusions as well.
