package context

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebsocketUpgradeAndOrigin(t *testing.T) {
	defer SetWebsocketCheckOrigin(nil)
	SetWebsocketCheckOrigin(func(r *http.Request) bool { return true })

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := New(w, r)
		conn, err := c.Upgrade()
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("pong"))
	}))
	defer up.Close()

	u := "ws" + strings.TrimPrefix(up.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, msg, err := conn.ReadMessage()
	if err != nil || string(msg) != "pong" {
		t.Fatalf("%s %v", msg, err)
	}
	SetWebsocketCheckOrigin(nil)
}
