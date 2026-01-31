# WebSockets 🔌

KVolt provides native support for WebSockets via `gorilla/websocket` integration.

## Usage

Use `c.Upgrade()` to promote an HTTP request to a WebSocket connection.


## Real-World Example: Simple Chat Hub

Here is how you can manage multiple unique connections and broadcast messages.

```go
package main

import (
    "log"
    "sync"
    "github.com/go-kvolt/kvolt"
    "github.com/go-kvolt/kvolt/context"
    "github.com/gorilla/websocket"
)

// Hub maintains the set of active clients
type Hub struct {
    clients    map[*websocket.Conn]bool
    broadcast  chan []byte
    register   chan *websocket.Conn
    unregister chan *websocket.Conn
    mu         sync.Mutex
}

func newHub() *Hub {
    return &Hub{
        clients:    make(map[*websocket.Conn]bool),
        broadcast:  make(chan []byte),
        register:   make(chan *websocket.Conn),
        unregister: make(chan *websocket.Conn),
    }
}

func (h *Hub) run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client] = true
            h.mu.Unlock()
        case client := <-h.unregister:
            if _, ok := h.clients[client]; ok {
                h.mu.Lock()
                delete(h.clients, client)
                client.Close()
                h.mu.Unlock()
            }
        case message := <-h.broadcast:
            h.mu.Lock()
            for client := range h.clients {
                err := client.WriteMessage(websocket.TextMessage, message)
                if err != nil {
                    client.Close()
                    delete(h.clients, client)
                }
            }
            h.mu.Unlock()
        }
    }
}

func main() {
    app := kvolt.New()
    hub := newHub()
    go hub.run()

    app.GET("/ws", func(c *context.Context) error {
        conn, err := c.Upgrade()
        if err != nil {
            return nil
        }
        
        hub.register <- conn
        
        // Read Loop
        for {
            _, message, err := conn.ReadMessage()
            if err != nil {
                hub.unregister <- conn
                break
            }
            // Broadcast received message
            hub.broadcast <- message
        }
        return nil
    })

    app.Run(":8080")
}
```

