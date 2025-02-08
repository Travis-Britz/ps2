// Package wsc implements a WebSocket Client for Planetside 2's event streaming service.
package wsc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/event"
	"github.com/gorilla/websocket"
)

func New(serviceID string, env ps2.Environment) *Client {
	c := &Client{
		messageLogger: &noopMessageLogger{},
		serviceID:     serviceID,
		env:           env,
	}
	return c
}

type Client struct {
	conn                          *websocket.Conn
	messageLogger                 messageLogger
	serviceID                     string
	env                           ps2.Environment
	serviceURL                    string
	err                           chan error
	connectHandler                func()
	playerLoginHandlers           []func(event.PlayerLogin)
	playerLogoutHandlers          []func(event.PlayerLogout)
	gainExperienceHandlers        []func(event.GainExperience)
	vehicleDestroyHandlers        []func(event.VehicleDestroy)
	deathHandlers                 []func(event.Death)
	achievementEarnedHandlers     []func(event.AchievementEarned)
	battleRankUpHandlers          []func(event.BattleRankUp)
	itemAddedHandlers             []func(event.ItemAdded)
	metagameEventHandlers         []func(event.MetagameEvent)
	facilityControlHandlers       []func(event.FacilityControl)
	playerFacilityCaptureHandlers []func(event.PlayerFacilityCapture)
	playerFacilityDefendHandlers  []func(event.PlayerFacilityDefend)
	skillAddedHandlers            []func(event.SkillAdded)
	continentLockHandlers         []func(event.ContinentLock)
	fishScanHandlers              []func(event.FishScan)
}

// SetMessageLogger sets a logger to track all sent and received websocket messages.
// This may be useful for debugging purposes or to generate replayable event streams for testing.
// Messages will be given to the logger before unmarshaling (for received messages).
// Given byte slices MUST NOT be modified in any way.
func (c *Client) SetMessageLogger(l messageLogger) {
	c.messageLogger = l
}

// SetURL allows overriding the default url for the event streaming service.
//
// This is useful if you would like to use a service like https://nanite-systems.net/ instead,
// which wraps the official Census event streaming API.
//
// Note that the provided serviceID and env in the constructor will be ignored when using SetURL.
func (c *Client) SetURL(url string) {
	c.serviceURL = url
}

// Run will connect and run the websocket client,
// blocking until ctx is cancelled or a connection error occurs.
//
// The returned error will be nil if the given context was cancelled or the deadline exceeded.
// Use [wsc.WithRetry] to reconnect on error.
func (c *Client) Run(ctx context.Context) error {
	ctx, shutdown := context.WithCancel(ctx)
	defer shutdown()
	url := c.url()
	dialer := websocket.Dialer{
		// Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 10 * time.Second,
	}
	slog.Debug("dialing event service", "url", url)
	conn, _, err := dialer.DialContext(ctx, url, nil)
	// conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("wsc.Client.Run: unable to connect: %w", err)
	}
	defer conn.Close()
	c.conn = conn
	if c.connectHandler != nil {
		c.connectHandler()
	}
	c.err = make(chan error, 1)
	messages := make(chan rawMessage, 100)
	go c.handle(ctx, messages)
	go c.read(ctx, messages)

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-c.err:
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}

// Send writes a subscription message to the event push service.
//
// Example:
//
//	sub := wsc.Subscribe{}
//	sub.AllWorlds()
//	sub.AllCharacters()
//	sub.AllEvents()
//	client.Send(sub)
func (c *Client) Send(cs commander) {
	b, err := json.Marshal(cs.command())
	if err != nil {
		slog.Error("error marshaling command to JSON", "error", err, "command", cs)
		return
	}
	c.messageLogger.Sent(b)
	if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
		c.exit(fmt.Errorf("write error: %w", err))
		return
	}
}

func (c *Client) read(ctx context.Context, messages chan<- rawMessage) {
	defer close(messages)
	var message []byte
	var err error
	messageLogger := c.messageLogger
	for {
		m := rawMessage{}
		_, message, err = c.conn.ReadMessage()
		if err != nil {
			c.exit(fmt.Errorf("read: %w", err))
			break
		}
		messageLogger.Received(message)
		err = json.Unmarshal(message, &m)
		if err != nil {
			slog.Error("decoding JSON failed", "error", err, "raw", string(message))
			continue
		}
		messages <- m
	}
}

// SetConnectHandler sets a function h to be called upon connect success.
func (c *Client) SetConnectHandler(h func()) {
	c.connectHandler = h
}

func (c *Client) AddHandler(h any) {
	switch v := h.(type) {
	case func(event.PlayerLogin):
		c.playerLoginHandlers = append(c.playerLoginHandlers, v)
	case func(event.PlayerLogout):
		c.playerLogoutHandlers = append(c.playerLogoutHandlers, v)
	case func(event.GainExperience):
		c.gainExperienceHandlers = append(c.gainExperienceHandlers, v)
	case func(event.VehicleDestroy):
		c.vehicleDestroyHandlers = append(c.vehicleDestroyHandlers, v)
	case func(event.Death):
		c.deathHandlers = append(c.deathHandlers, v)
	case func(event.AchievementEarned):
		c.achievementEarnedHandlers = append(c.achievementEarnedHandlers, v)
	case func(event.BattleRankUp):
		c.battleRankUpHandlers = append(c.battleRankUpHandlers, v)
	case func(event.ItemAdded):
		c.itemAddedHandlers = append(c.itemAddedHandlers, v)
	case func(event.MetagameEvent):
		c.metagameEventHandlers = append(c.metagameEventHandlers, v)
	case func(event.FacilityControl):
		c.facilityControlHandlers = append(c.facilityControlHandlers, v)
	case func(event.PlayerFacilityCapture):
		c.playerFacilityCaptureHandlers = append(c.playerFacilityCaptureHandlers, v)
	case func(event.PlayerFacilityDefend):
		c.playerFacilityDefendHandlers = append(c.playerFacilityDefendHandlers, v)
	case func(event.SkillAdded):
		c.skillAddedHandlers = append(c.skillAddedHandlers, v)
	case func(event.ContinentLock):
		c.continentLockHandlers = append(c.continentLockHandlers, v)
	case func(event.FishScan):
		c.fishScanHandlers = append(c.fishScanHandlers, v)
	default:
		panic(fmt.Sprintf("AddHandler: invalid type '%T'", h))
	}
}

func (c *Client) handle(ctx context.Context, messages <-chan rawMessage) {
	// dedup := make(deduplicator, 0, 10000)
	for m := range messages {
		e := m.message()
		// if ee, ok := e.(uniqueTimestampedEvent); ok {
		// 	if !dedup.InsertFresh(ee) {
		// 		slog.Debug("duplicate event dropped", "event", e)
		// 		continue
		// 	}
		// }
		switch v := e.(type) {
		case event.PlayerLogin:
			for _, h := range c.playerLoginHandlers {
				h(v)
			}
		case event.PlayerLogout:
			for _, h := range c.playerLogoutHandlers {
				h(v)
			}
		case event.GainExperience:
			for _, h := range c.gainExperienceHandlers {
				h(v)
			}
		case event.VehicleDestroy:
			for _, h := range c.vehicleDestroyHandlers {
				h(v)
			}
		case event.Death:
			for _, h := range c.deathHandlers {
				h(v)
			}
		case event.AchievementEarned:
			for _, h := range c.achievementEarnedHandlers {
				h(v)
			}
		case event.BattleRankUp:
			for _, h := range c.battleRankUpHandlers {
				h(v)
			}
		case event.ItemAdded:
			for _, h := range c.itemAddedHandlers {
				h(v)
			}
		case event.MetagameEvent:
			for _, h := range c.metagameEventHandlers {
				h(v)
			}
		case event.FacilityControl:
			for _, h := range c.facilityControlHandlers {
				h(v)
			}
		case event.PlayerFacilityCapture:
			for _, h := range c.playerFacilityCaptureHandlers {
				h(v)
			}
		case event.PlayerFacilityDefend:
			for _, h := range c.playerFacilityDefendHandlers {
				h(v)
			}
		case event.SkillAdded:
			for _, h := range c.skillAddedHandlers {
				h(v)
			}
		case event.ContinentLock:
			for _, h := range c.continentLockHandlers {
				h(v)
			}
		}
	}
}

func (c *Client) url() string {
	if c.serviceURL != "" {
		return c.serviceURL
	}
	return fmt.Sprintf("wss://push.planetside2.com/streaming?environment=%s&service-id=s:%s", c.env, url.QueryEscape(c.serviceID))
}

func (c *Client) debug(fmt string, v ...any) {
	log.Printf(fmt, v...)
}

// exit signals the client to stop with err.
func (c *Client) exit(err error) {
	select {
	case c.err <- err:
	default:
	}
}

type messageLogger interface {
	Sent([]byte)
	Received([]byte)
}

// noopMessageLogger performs no op.
type noopMessageLogger struct{}

func (noopMessageLogger) Sent([]byte)     {}
func (noopMessageLogger) Received([]byte) {}

// MessageLogger implements the messageLogger interface accepted by [client.SetMessageLogger].
// R and S can be the same writer.
// Writers cannot be nil; use io.Discard instead.
type MessageLogger struct {
	R  io.Writer // Writer for messages received by the client
	S  io.Writer // Writer for outgoing messages sent by the client
	mu sync.Mutex

	SentPrefix     string
	ReceivedPrefix string
}

func (l *MessageLogger) Sent(b []byte) {
	l.mu.Lock()
	fmt.Fprintln(l.S, l.SentPrefix+string(b))
	l.mu.Unlock()
}
func (l *MessageLogger) Received(b []byte) {
	l.mu.Lock()
	fmt.Fprintln(l.R, l.ReceivedPrefix+string(b))
	l.mu.Unlock()
}
