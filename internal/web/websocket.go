package web

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	DefaultPingInterval = 30 * time.Second
	DefaultPongWait     = 60 * time.Second
	DefaultWriteWait    = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Conn struct {
	ws *websocket.Conn

	pingInterval time.Duration
	pongWait     time.Duration
	writeWait    time.Duration

	writeMu   sync.Mutex
	done      chan struct{}
	closeOnce sync.Once
}

type UpgradeOption func(*Conn)

func WithKeepalive(pingInterval, pongWait time.Duration) UpgradeOption {
	return func(c *Conn) {
		if pingInterval > 0 {
			c.pingInterval = pingInterval
		}
		if pongWait > 0 {
			c.pongWait = pongWait
		}
	}
}

func WithWriteWait(writeWait time.Duration) UpgradeOption {
	return func(c *Conn) {
		if writeWait > 0 {
			c.writeWait = writeWait
		}
	}
}

// Quem chama precisa ler da conexao para que os frames de pong sejam processados.
func Upgrade(w http.ResponseWriter, r *http.Request, opts ...UpgradeOption) (*Conn, error) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	c := &Conn{
		ws:           ws,
		pingInterval: DefaultPingInterval,
		pongWait:     DefaultPongWait,
		writeWait:    DefaultWriteWait,
		done:         make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}

	_ = ws.SetReadDeadline(time.Now().Add(c.pongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(c.pongWait))
	})

	go c.keepalive()
	return c, nil
}

func (c *Conn) keepalive() {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.writeMu.Lock()
			_ = c.ws.SetWriteDeadline(time.Now().Add(c.writeWait))
			err := c.ws.WriteMessage(websocket.PingMessage, nil)
			c.writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func (c *Conn) WriteJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.ws.SetWriteDeadline(time.Now().Add(c.writeWait)); err != nil {
		return err
	}
	return c.ws.WriteJSON(v)
}

func (c *Conn) ReadMessage() (int, []byte, error) {
	return c.ws.ReadMessage()
}

func (c *Conn) RemoteAddr() string {
	return c.ws.RemoteAddr().String()
}

func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)

		c.writeMu.Lock()
		_ = c.ws.SetWriteDeadline(time.Now().Add(c.writeWait))
		_ = c.ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.writeMu.Unlock()

		err = c.ws.Close()
	})
	return err
}
