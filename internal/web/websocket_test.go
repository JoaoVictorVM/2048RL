package web

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newWSServer(t *testing.T, handle func(*Conn), opts ...UpgradeOption) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r, opts...)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn)
	}))
	t.Cleanup(ts.Close)
	return "ws" + strings.TrimPrefix(ts.URL, "http")
}

func TestWebSocketUpgrade_Succeeds(t *testing.T) {
	received := make(chan string, 1)
	url := newWSServer(t, func(conn *Conn) {
		if err := conn.WriteJSON(map[string]string{"status": "ready"}); err != nil {
			t.Errorf("WriteJSON: %v", err)
		}
		<-received
	})

	client, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	var payload map[string]string
	if err := client.ReadJSON(&payload); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if payload["status"] != "ready" {
		t.Errorf("unexpected payload: %v", payload)
	}
	received <- payload["status"]
}

func TestWebSocketUpgrade_KeepaliveRespondsToPing(t *testing.T) {
	const (
		pingInterval = 20 * time.Millisecond
		pongWait     = 200 * time.Millisecond
	)

	readErr := make(chan error, 1)
	url := newWSServer(t, func(conn *Conn) {
		_, _, err := conn.ReadMessage()
		readErr <- err
	}, WithKeepalive(pingInterval, pongWait))

	client, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	clientErr := make(chan error, 1)
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				clientErr <- err
				return
			}
		}
	}()

	select {
	case err := <-readErr:
		t.Fatalf("server connection died before pongWait elapsed: %v", err)
	case err := <-clientErr:
		t.Fatalf("client connection died: %v", err)
	case <-time.After(3 * pongWait):
	}
}

func TestWebSocketUpgrade_GracefulClose(t *testing.T) {
	before := runtime.NumGoroutine()

	handlerDone := make(chan error, 1)
	url := newWSServer(t, func(conn *Conn) {
		_, _, err := conn.ReadMessage()
		handlerDone <- err
	}, WithKeepalive(20*time.Millisecond, time.Second))

	client, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	if err := client.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		t.Fatalf("write close: %v", err)
	}
	client.Close()

	select {
	case err := <-handlerDone:
		if err == nil {
			t.Fatal("expected the server read to return an error after the client closed")
		}
		if !websocket.IsCloseError(err, websocket.CloseNormalClosure) &&
			!websocket.IsUnexpectedCloseError(err) {
			t.Logf("close error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server handler did not return after the client closed")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("goroutines leaked: %d before, %d after", before, runtime.NumGoroutine())
}

func TestWebSocketConn_CloseIsIdempotent(t *testing.T) {
	url := newWSServer(t, func(conn *Conn) {
		if err := conn.Close(); err != nil {
			t.Errorf("first Close: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Errorf("second Close: %v", err)
		}
	})

	client, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	if _, _, err := client.ReadMessage(); err == nil {
		t.Fatal("expected the client read to fail after the server closed")
	}
}
