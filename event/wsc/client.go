// Package wsc implements a WebSocket Client for Planetside 2's event streaming service.
package wsc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/event"
	"github.com/gorilla/websocket"
)

func New(serviceID string, env ps2.Environment) *Client {
	c := &Client{
		serviceID: serviceID,
		env:       env,
	}
	return c
}

type Client struct {
	conn                          *websocket.Conn
	serviceID                     string
	env                           ps2.Environment
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
	continentUnlockHandlers       []func(event.ContinentUnlock)
}

func (c *Client) Run(ctx context.Context) error {
	ctx, shutdown := context.WithCancel(ctx)
	defer shutdown()
	url := c.url()
	c.debug("dialing: %v", url)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url(), nil)
	if err != nil {
		return err
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
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (c *Client) Send(cs commander) {
	b, err := json.Marshal(cs.command())
	if err != nil {
		log.Printf("send: %v", err)
		return
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
		c.exit(fmt.Errorf("write: %w", err))
	}
}

func (c *Client) read(ctx context.Context, messages chan<- rawMessage) {
	defer close(messages)
	var message []byte
	var err error
	var m rawMessage
	for {
		_, message, err = c.conn.ReadMessage()
		if err != nil {
			c.exit(fmt.Errorf("read: %w", err))
			break
		}
		err = json.Unmarshal(message, &m)
		if err != nil {
			c.debug("decode error: %v; raw message: %s", err, message)
			continue
		}
		messages <- m
	}
}

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
		c.skillAddedHandlers = append(c.skillAddedHandlers)
	case func(event.ContinentLock):
		c.continentLockHandlers = append(c.continentLockHandlers, v)
	case func(event.ContinentUnlock):
		c.continentUnlockHandlers = append(c.continentUnlockHandlers, v)
	default:
		panic(fmt.Sprintf("AddHandler: invalid type '%T'", h))
	}
}

func (c *Client) handle(ctx context.Context, messages <-chan rawMessage) {
	for m := range messages {
		e := m.message()
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
		case event.ContinentUnlock:
			for _, h := range c.continentUnlockHandlers {
				h(v)
			}
		}
	}
}

func (cc *Client) url() string {
	return fmt.Sprintf("wss://push.planetside2.com/streaming?environment=%s&service-id=s:%s", cc.env, cc.serviceID)
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
