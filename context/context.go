package context

import (
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"os"

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

	// Keys is a key/value pair exclusively for the context of each request.
	Keys map[string]interface{}

	// Templates holds the parsed templates (injected by Engine)
	Templates *template.Template
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
	c.Keys = nil
	c.Templates = nil // Reset templates
	c.index = -1
	c.headerWritten = false
}

// Set is used to store a new key/value pair exclusively for this context.
// It also lazily initializes  c.Keys if it was not used previously.
func (c *Context) Set(key string, value interface{}) {
	if c.Keys == nil {
		c.Keys = make(map[string]interface{})
	}
	c.Keys[key] = value
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exist it returns (nil, false)
func (c *Context) Get(key string) (value interface{}, exists bool) {
	if c.Keys != nil {
		value, exists = c.Keys[key]
	}
	return
}

// MustGet returns the value for the given key if it exists, otherwise it panics.
func (c *Context) MustGet(key string) interface{} {
	if value, exists := c.Get(key); exists {
		return value
	}
	panic("Key \"" + key + "\" does not exist")
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

// RenderHTML renders the template with data and sets values content-type to "text/html".
func (c *Context) RenderHTML(code int, name string, data interface{}) error {
	c.Status(code)
	c.Writer.Header().Set("Content-Type", "text/html")
	if c.Templates == nil {
		return c.String(500, "Templates not loaded")
	}
	return c.Templates.ExecuteTemplate(c.Writer, name, data)
}

// HTML sends an HTML response (Raw String).
func (c *Context) HTML(code int, html string) error {
	c.Writer.Header().Set("Content-Type", "text/html")
	if !c.headerWritten {
		c.Writer.WriteHeader(code)
		c.headerWritten = true
	}
	_, err := c.Writer.Write([]byte(html))
	return err
}

// FormFile returns the first file for the provided form key.
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32MB default
			return nil, err
		}
	}
	f, fh, err := c.Request.FormFile(name)
	if err != nil {
		return nil, err
	}
	f.Close()
	return fh, nil
}

// SaveUploadedFile uploads the form file to specific dst.
func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// File writes the specified file into the body stream in an efficient way.
func (c *Context) File(filepath string) {
	http.ServeFile(c.Writer, c.Request, filepath)
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
