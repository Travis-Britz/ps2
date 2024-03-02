# About

`eventclient` is a CLI tool to connect to the Planetside 2 event streaming service and print received events to the command line.
This tool is only meant for simple use cases like collecting event samples to experiment with.

## Installation

```sh
git clone git@github.com:Travis-Britz/ps2.git
cd ps2
go build ./cmd/eventclient
./eventclient.exe -h
```

Alternatively, using [go install](https://go.dev/ref/mod#go-install):

```sh
go install github.com/Travis-Britz/ps2/cmd/eventclient
eventclient -h
```

## Usage

`./eventclient -h`:

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
Use shell redirection to send output to a file to create logs.

Expected parameters are _either_ a WorldID or set of character names,
but not both.
Leaving both empty will track all events for all worlds.

For example:

```
./eventclient -world 17 >> Emerald.log
```

```
./eventclient -player higby >> higby.log
```

```sh
# very useful for watching how events interact between squad members and vehicle players
./eventclient -player higby -player wrel >> combined.log
```

```
./eventclient >> everything.log
```

If you want more than one specific world,
but not all worlds,
then run the program multiple times in separate processes.

Various information may be sent to stderr as well.
What is shown on stderr will probably change over time.
Currently it displays parsed event structs and debug information.

Rendering thousands of debug lines to the terminal can be cpu-intensive.
If you don't care about the stderr messages,
just redirect stderr to `/dev/null` (or `NUL`).

```
./eventclient -world 17 >> Emerald.log 2> /dev/null
```

## Working With Logs

I _highly_ recommend using `jq` for searching generated log files: https://jqlang.github.io/jq/

Video introduction to `jq`: https://www.youtube.com/watch?v=n8sOmEe2SDg

### Example `jq`

Find death events where there is an attacker ID,
but the attacker team was 0:

```sh
jq 'select(.payload.event_name == "Death" and .payload.attacker_character_id != "0" and .payload.attacker_team_id == "0") | .payload' everything.log

```

Searching a log with 673k events found only four matches:

```log
{"attacker_character_id":"5429421374730973281","attacker_fire_mode_id":"0","attacker_loadout_id":"0","attacker_team_id":"0","attacker_vehicle_id":"151","attacker_weapon_id":"0","character_id":"5429423912994111185","character_loadout_id":"6","event_name":"Death","is_critical":"0","is_headshot":"0","team_id":"2","timestamp":"1709037290","world_id":"40","zone_id":"8"}
{"attacker_character_id":"5429421374730973281","attacker_fire_mode_id":"0","attacker_loadout_id":"0","attacker_team_id":"0","attacker_vehicle_id":"151","attacker_weapon_id":"0","character_id":"5429320287649428417","character_loadout_id":"5","event_name":"Death","is_critical":"0","is_headshot":"0","team_id":"2","timestamp":"1709037290","world_id":"40","zone_id":"8"}
{"attacker_character_id":"5429421374730973281","attacker_fire_mode_id":"0","attacker_loadout_id":"0","attacker_team_id":"0","attacker_vehicle_id":"151","attacker_weapon_id":"0","character_id":"5429056089889202177","character_loadout_id":"5","event_name":"Death","is_critical":"0","is_headshot":"0","team_id":"2","timestamp":"1709037290","world_id":"40","zone_id":"8"}
{"attacker_character_id":"5428779225270793185","attacker_fire_mode_id":"0","attacker_loadout_id":"0","attacker_team_id":"0","attacker_vehicle_id":"0","attacker_weapon_id":"6002526","character_id":"5428345446433382657","character_loadout_id":"17","event_name":"Death","is_critical":"0","is_headshot":"0","team_id":"1","timestamp":"1709043979","world_id":"17","zone_id":"6"}
```

## TLS failures

Go 1.22 dropped support for a number of TLS key exchange ciphers that are used by the census event push service.

If the program exits with "tls: handshake failure",
then set `GODEBUG=tlsrsakex=1`.

Bash:

```sh
GODEBUG=tlsrsakex=1 ./eventclient [flags] > [file.log]
```

PowerShell:

```ps
$env:GODEBUG='tlsrsakex=1' ; .\eventclient.exe [flags] > [file.log]
```
