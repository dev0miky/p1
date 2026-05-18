package esl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/percipia/eslgo"
	"github.com/percipia/eslgo/command"
)

type Handler func(Event)

type Client struct {
	addr     string
	password string
	logger   *slog.Logger
	conn     atomic.Pointer[eslgo.Conn]
	mu       sync.Mutex
	handler  Handler
	closed   atomic.Bool
}

func Dial(ctx context.Context, addr, password string, logger *slog.Logger) (*Client, error) {
	c := &Client{addr: addr, password: password, logger: logger}
	if err := c.connect(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) connect(ctx context.Context) error {
	conn, err := eslgo.Dial(c.addr, c.password, c.onDisconnect)
	if err != nil {
		return fmt.Errorf("esl dial %s: %w", c.addr, err)
	}
	c.conn.Store(conn)
	if c.handler != nil {
		conn.RegisterEventListener(eslgo.EventListenAll, c.dispatch)
	}
	c.logger.Info("esl connected", "addr", c.addr)
	_ = ctx
	return nil
}

func (c *Client) onDisconnect() {
	if c.closed.Load() {
		return
	}
	c.logger.Warn("esl disconnected, will reconnect")
	go c.reconnectLoop()
}

func (c *Client) reconnectLoop() {
	backoff := 500 * time.Millisecond
	for !c.closed.Load() {
		time.Sleep(backoff)
		if err := c.connect(context.Background()); err != nil {
			c.logger.Warn("esl reconnect failed", "err", err, "backoff", backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}
		return
	}
}

func (c *Client) OnEvent(h Handler) {
	c.mu.Lock()
	c.handler = h
	c.mu.Unlock()
	if conn := c.conn.Load(); conn != nil {
		conn.RegisterEventListener(eslgo.EventListenAll, c.dispatch)
	}
}

func (c *Client) dispatch(evt *eslgo.Event) {
	c.mu.Lock()
	h := c.handler
	c.mu.Unlock()
	if h == nil {
		return
	}
	headers := make(map[string]string, len(evt.Headers))
	for k, vals := range evt.Headers {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	h(ParseEvent(headers))
}

func (c *Client) Subscribe(ctx context.Context, events ...string) error {
	conn := c.conn.Load()
	if conn == nil {
		return errors.New("esl: not connected")
	}
	res, err := conn.SendCommand(ctx, command.Event{Format: "plain", Listen: events})
	if err != nil {
		return err
	}
	if !res.IsOk() {
		return fmt.Errorf("subscribe: %s", string(res.Body))
	}
	c.logger.Info("esl subscribed", "events", strings.Join(events, ","))
	return nil
}

func (c *Client) API(ctx context.Context, cmd string) (string, error) {
	conn := c.conn.Load()
	if conn == nil {
		return "", errors.New("esl: not connected")
	}
	res, err := conn.SendCommand(ctx, command.API{Command: cmd})
	if err != nil {
		return "", err
	}
	return string(res.Body), nil
}

func (c *Client) BgAPI(ctx context.Context, cmd string) (jobUUID string, err error) {
	conn := c.conn.Load()
	if conn == nil {
		return "", errors.New("esl: not connected")
	}
	res, err := conn.SendCommand(ctx, command.API{Command: cmd, Background: true})
	if err != nil {
		return "", err
	}
	return res.Headers.Get("Job-UUID"), nil
}

type OriginateParams struct {
	Vars    OriginateVars
	Gateway string
	Dest    string
	Action  string
}

func (c *Client) Originate(ctx context.Context, p OriginateParams) (jobUUID string, err error) {
	if p.Gateway == "" {
		return "", errors.New("originate: gateway required")
	}
	if p.Dest == "" {
		return "", errors.New("originate: dest required")
	}
	action := p.Action
	if action == "" {
		action = "&park"
	}
	cmd := fmt.Sprintf("%ssofia/gateway/%s/%s %s", p.Vars, p.Gateway, p.Dest, action)
	return c.BgAPI(ctx, cmd)
}

func (c *Client) Close() {
	c.closed.Store(true)
	if conn := c.conn.Load(); conn != nil {
		conn.ExitAndClose()
	}
}
