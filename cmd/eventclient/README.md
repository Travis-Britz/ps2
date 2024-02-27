# eventclient

Eventclient is a CLI tool to connect to the Planetside 2 event streaming service and print events to the command line.
This tool is only meant for simple use cases like collecting event samples to experiment with.

    Usage of eventclient:
    -env string
            Environment (pc, ps4us, ps4eu) (default "pc")
    -player string
            Character name to track
    -sid string
            Planetside Census Service ID (default "example")
    -world int
            World ID to subscribe to

Events are sent to stdout.
Use standard shell redirection to send output to a file to create logs.

For example:

```
./eventclient -world 17 >> Emerald.log
```

```
./eventclient -player higby >> higby.log
```

```
./eventclient -player higby -player wrel >> combined.log
```

Various information may be sent to stderr as well.
What is shown will probably change over time.
Currently it displays parsed event structs and debug information.

Expected parameters are _either_ a WorldID or set of character names,
but not both.
Leaving both empty will track all events for all worlds.
