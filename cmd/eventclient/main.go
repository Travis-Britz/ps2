package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
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
	flag.StringVar(&config.PlanetsideCensusServiceID, "sid", config.PlanetsideCensusServiceID, "Planetside Census Service ID")
	flag.StringVar(&envString, "env", "pc", "Environment (pc, ps4us, ps4eu)")
	flag.Var(&players, "player", "Player to track")
	flag.IntVar(&world, "world", 0, "World ID to subscribe to")
	flag.Parse()

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
			log.Fatalf("failed to look up character ID for \"%s\": %v", p, err)
		}
		config.PlanetsideCharacterIDs = append(config.PlanetsideCharacterIDs, cid)
	}
}

func run() (err error) {
	client := wsc.New(config.PlanetsideCensusServiceID, config.PlanetsideEnvironment)
	ctx, shutdown := context.WithCancel(context.Background())
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		log.Println("interrupt")
		shutdown()
	}()

	subscribe := new(wsc.Subscribe)
	subscribe.AllEvents()

	switch {
	case len(config.PlanetsideCharacterIDs) > 0:
		subscribe.Characters = config.PlanetsideCharacterIDs
	case config.PlanetsideWorldID != 0:
		subscribe.AddWorld(config.PlanetsideWorldID)
		subscribe.AllCharacters()
	default:
		subscribe.AllCharacters()
		subscribe.AllWorlds()
	}

	client.SetConnectHandler(func() {
		client.Send(subscribe)
	})

	client.AddHandler(func(e event.ContinentLock) {
		display(e)
	})
	client.AddHandler(func(e event.PlayerLogin) {
		display(e)
	})
	client.AddHandler(func(e event.PlayerLogout) {
		display(e)
	})
	client.AddHandler(func(e event.GainExperience) {
		display(e)
	})
	client.AddHandler(func(e event.VehicleDestroy) {
		display(e)
	})
	client.AddHandler(func(e event.Death) {
		display(e)
	})
	client.AddHandler(func(e event.AchievementEarned) {
		display(e)
	})
	client.AddHandler(func(e event.BattleRankUp) {
		display(e)
	})
	client.AddHandler(func(e event.ItemAdded) {
		display(e)
	})
	client.AddHandler(func(e event.MetagameEvent) {
		display(e)
	})
	client.AddHandler(func(e event.FacilityControl) {
		display(e)
	})
	client.AddHandler(func(e event.PlayerFacilityCapture) {
		display(e)
	})
	client.AddHandler(func(e event.PlayerFacilityDefend) {
		display(e)
	})
	client.AddHandler(func(e event.SkillAdded) {
		display(e)
	})
	client.SetMessageLogger(&wsc.MessageLogger{R: os.Stdout, S: os.Stderr, SentPrefix: "-> "})
	err = client.Run(ctx)
	return err
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func display(m any) {
	log.Printf("%#v\n", m)
	// b, err := json.Marshal(m)
	// if err != nil {
	// 	return
	// }
	// log.Println(string(b))
}

type characterArray []string

func (i *characterArray) Set(value string) error {
	*i = append(*i, value)
	return nil
}
func (i *characterArray) String() string {
	return strings.Join(*i, ",")
}
