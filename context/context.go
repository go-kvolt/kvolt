package context

import (
	"encoding/json"
	"net/http"
)

// HandlerFunc matches the KVolt handler signature.
type HandlerFunc func(*Context) error

// Context is the context for the current request.
// It wraps http.ResponseWriter and *http.Request and adds helper methods.
type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	// Handlers is the middleware chain for this request
	Handlers []HandlerFunc

	// index is the current middleware index
	index int
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
	c.index = -1
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
	c.Writer.WriteHeader(code)
	return c
}

// JSON sends a JSON response.
func (c *Context) JSON(code int, obj interface{}) error {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	return json.NewEncoder(c.Writer).Encode(obj)
}

// String sends a plain text response.
func (c *Context) String(code int, format string, values ...interface{}) error {
	c.Writer.Header().Set("Content-Type", "text/plain")
	c.Writer.WriteHeader(code)
	_, err := c.Writer.Write([]byte(format))
	return err
}
