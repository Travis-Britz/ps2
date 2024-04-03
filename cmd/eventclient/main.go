package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/census"
	"github.com/Travis-Britz/ps2/event"
	"github.com/Travis-Britz/ps2/event/wsc"
)

var config = struct {
	PlanetsideCensusServiceID string
	PlanetsideEnvironment     ps2.Environment
	PlanetsideCharacterIDs    []ps2.CharacterID
	PlanetsideWorldID         ps2.WorldID
}{
	PlanetsideCensusServiceID: "example",
}

func init() {
	var envString string
	var players characterArray
	var world int
	var verbose bool
	flag.StringVar(&config.PlanetsideCensusServiceID, "sid", config.PlanetsideCensusServiceID, "Planetside Census Service ID")
	flag.StringVar(&envString, "env", "pc", "Environment (pc, ps4us, ps4eu)")
	flag.Var(&players, "player", "Player to track")
	flag.IntVar(&world, "world", 0, "World ID to subscribe to")
	flag.BoolVar(&verbose, "v", false, "Enable verbose log output")
	flag.Parse()

	if verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	config.PlanetsideWorldID = ps2.WorldID(world)
	censusClient := &census.Client{Key: config.PlanetsideCensusServiceID}

	switch envString {
	case "pc":
		config.PlanetsideEnvironment = ps2.PC
	case "ps4us":
		config.PlanetsideEnvironment = ps2.PS4US
	case "ps4eu":
		config.PlanetsideEnvironment = ps2.PS4EU
	}

	for _, p := range players {
		cid, err := census.GetCharacterIDByName(context.Background(), censusClient, config.PlanetsideEnvironment, p)
		if err != nil {
			log.Fatalf("failed to look up character ID for %q: %v", p, err)
		}
		config.PlanetsideCharacterIDs = append(config.PlanetsideCharacterIDs, cid)
	}
}

func main() {
	ctx, shutdown := context.WithCancelCause(context.Background())
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		slog.Info("received interrupt")
		shutdown(errors.New("exiting normally"))
	}()
	if err := run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			err = context.Cause(ctx)
			slog.Info(err.Error())
			return
		}
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) (err error) {
	client := wsc.New(config.PlanetsideCensusServiceID, config.PlanetsideEnvironment)

	subscribe := new(wsc.Subscribe)
	subscribe.AllEvents()

	switch {
	case len(config.PlanetsideCharacterIDs) > 0:
		subscribe.Characters = config.PlanetsideCharacterIDs
	case config.PlanetsideWorldID != 0:
		subscribe.AddWorld(config.PlanetsideWorldID)
		subscribe.AllCharacters()
		subscribe.LogicalAndCharactersWithWorlds = true
	default:
		subscribe.AllCharacters()
		subscribe.AllWorlds()
	}

	client.SetConnectHandler(func() {
		slog.Info("websocket connected")
		client.Send(subscribe)
	})

	client.AddHandler(func(e event.ContinentLock) { display(e) })
	client.AddHandler(func(e event.PlayerLogin) { display(e) })
	client.AddHandler(func(e event.PlayerLogout) { display(e) })
	client.AddHandler(func(e event.GainExperience) { display(e) })
	client.AddHandler(func(e event.VehicleDestroy) { display(e) })
	client.AddHandler(func(e event.Death) { display(e) })
	client.AddHandler(func(e event.AchievementEarned) { display(e) })
	client.AddHandler(func(e event.BattleRankUp) { display(e) })
	client.AddHandler(func(e event.ItemAdded) { display(e) })
	client.AddHandler(func(e event.MetagameEvent) { display(e) })
	client.AddHandler(func(e event.FacilityControl) { display(e) })
	client.AddHandler(func(e event.PlayerFacilityCapture) { display(e) })
	client.AddHandler(func(e event.PlayerFacilityDefend) { display(e) })
	client.AddHandler(func(e event.SkillAdded) { display(e) })

	var writer io.Writer = os.Stdout

	// If stdout is going to the terminal then we'll also create a log file for the connection.
	// The number of times I've had a log printed to the terminal and then wanted to go back and search it later is greater than zero,
	// and the storage is minimal.
	// The websocket event service has a compression ratio of ~94%;
	// 1 GB of logs should compress to about 60MB.
	// Also, since we're writing the log to a system temp dir,
	// most systems will have some mechanism to clean up that dir eventually.
	//
	// If stdout is being redirected then there is no need to create a full log file.
	o, _ := os.Stdout.Stat()
	if (o.Mode() & os.ModeCharDevice) == os.ModeCharDevice {
		logDir := filepath.Join(os.TempDir(), "census-event-logs")
		if err := os.MkdirAll(logDir, 0700); err != nil {
			return fmt.Errorf("failed to create dir: %w", err)
		}

		logFile, err := os.CreateTemp(logDir, "websocket-*.log.gz")
		if err != nil {
			return fmt.Errorf("log file creation failed: %w", err)
		}
		defer slog.Info("a full session websocket log is available", "filepath", logFile.Name())
		defer logFile.Close()

		// a buffer size of 10MB for gzipped logs will usually not even write until exit
		buf := bufio.NewWriterSize(logFile, 1024*10_000)
		defer buf.Flush()

		gzippedLog, _ := gzip.NewWriterLevel(buf, gzip.BestCompression)
		defer gzippedLog.Close()

		writer = io.MultiWriter(os.Stdout, gzippedLog)
	}

	client.SetMessageLogger(&wsc.MessageLogger{R: writer, S: os.Stderr, SentPrefix: "-> "})
	err = client.Run(ctx)
	return err
}

func display(m any) {
	e := m.(event.Timestamper)
	slog.Debug("event", fmt.Sprintf("%T", m), m, "time_unix", e.Time().Unix())
}

type characterArray []string

func (i *characterArray) Set(value string) error {
	*i = append(*i, value)
	return nil
}
func (i *characterArray) String() string {
	return strings.Join(*i, ",")
}
