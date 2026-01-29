package context

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/go-kvolt/kvolt/router"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
)

// validate holds the global validator instance.
var validate = validator.New()

// HandlerFunc matches the KVolt handler signature.
type HandlerFunc func(*Context) error

// Context is the context for the current request.
// It wraps http.ResponseWriter and *http.Request and adds helper methods.
type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	// Handlers is the middleware chain for this request
	Handlers []HandlerFunc

	// Params are the route parameters
	Params router.Params

	// index is the current middleware index
	index int

	// headerWritten ensures we don't write headers twice
	headerWritten bool
}

// New creates a new Context.
func New(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Writer:  w,
		Request: r,
		index:   -1,
	}
}

// Reset re-initializes the context for a new request.
// Crucial for sync.Pool reuse.
func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	c.Writer = w
	c.Request = r
	c.Handlers = nil
	c.Params = nil
	c.index = -1
	c.headerWritten = false
}

// Param returns the value of the URL param.
func (c *Context) Param(key string) string {
	return c.Params.Get(key)
}

// Bind decodes the request body into obj and validates it.
// Currently supports JSON.
func (c *Context) Bind(obj interface{}) error {
	// 1. Decode JSON
	// We assume JSON by default or if Content-Type is application/json
	if err := sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(obj); err != nil {
		return err
	}

	// 2. Validate
	return validate.Struct(obj)
}

// Next executes the next middleware in the chain.
func (c *Context) Next() {
	c.index++
	if c.index < len(c.Handlers) {
		handler := c.Handlers[c.index]
		if err := handler(c); err != nil {
			// Error handling stub
		}
	}
}

// Status sets the HTTP status code.
func (c *Context) Status(code int) *Context {
	if !c.headerWritten {
		c.Writer.WriteHeader(code)
		c.headerWritten = true
	}
	return c
}

// JSON sends a JSON response.
func (c *Context) JSON(code int, obj interface{}) error {
	c.Writer.Header().Set("Content-Type", "application/json")
	if !c.headerWritten {
		c.Writer.WriteHeader(code)
		c.headerWritten = true
	}
	return sonic.ConfigDefault.NewEncoder(c.Writer).Encode(obj)
}

// String sends a plain text response.
func (c *Context) String(code int, format string, values ...interface{}) error {
	c.Writer.Header().Set("Content-Type", "text/plain")
	if !c.headerWritten {
		c.Writer.WriteHeader(code)
		c.headerWritten = true
	}
	_, err := c.Writer.Write([]byte(format))
	return err
}

// HTML sends an HTML response.
func (c *Context) HTML(code int, html string) error {
	c.Writer.Header().Set("Content-Type", "text/html")
	if !c.headerWritten {
		c.Writer.WriteHeader(code)
		c.headerWritten = true
	}
	_, err := c.Writer.Write([]byte(html))
	return err
}

var upgrader = websocket.Upgrader{ // Default upgrader
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity
	},
}

// Upgrade upgrades the HTTP connection to a WebSocket connection.
// Returns the *websocket.Conn and any error.
func (c *Context) Upgrade() (*websocket.Conn, error) {
	return upgrader.Upgrade(c.Writer, c.Request, nil)
}
