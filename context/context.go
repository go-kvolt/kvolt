package context

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"

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

	// status is the HTTP status to write. 0 means unset (defaults to 200 on first write).
	status int

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

// HeaderWritten reports whether the response headers have been sent.
// Used by middleware (e.g. Recovery) to avoid writing after response started.
func (c *Context) HeaderWritten() bool {
	return c.headerWritten
}

// StatusCode returns the status that will be / was written. 0 if nothing was set yet.
func (c *Context) StatusCode() int {
	if c.status != 0 {
		return c.status
	}
	if c.headerWritten {
		return http.StatusOK
	}
	return 0
}

// Reset re-initializes the context for a new request.
// Params backing array and Keys map are kept to cut allocations.
func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	c.Writer = w
	c.Request = r
	c.Handlers = nil
	c.Params = c.Params[:0]
	if c.Keys != nil {
		clear(c.Keys)
	}
	c.Templates = nil
	c.index = -1
	c.status = 0
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

// Query returns the first query string value for key (e.g. /search?q=foo).
func (c *Context) Query(key string) string {
	if c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return c.Request.URL.Query().Get(key)
}

// Bind decodes the JSON body into obj and then validates it (go-playground tags).
func (c *Context) Bind(obj interface{}) error {
	if err := c.BindJSON(obj); err != nil {
		return err
	}
	return validate.Struct(obj)
}

// Next executes the next middleware in the chain.
// If a handler returns an error and no response has been written yet,
// a 500 Internal Server Error is sent and the chain is stopped.
func (c *Context) Next() {
	c.index++
	if c.index < len(c.Handlers) {
		handler := c.Handlers[c.index]
		if err := handler(c); err != nil {
			log.Printf("[KVolt] handler error: %v", err)
			if !c.headerWritten {
				c.writeJSONBytes(http.StatusInternalServerError, errInternalJSON)
			}
			return
		}
	}
}

// Status stores the HTTP status code. Headers are not written until the body is sent,
// so Content-Type can still be set afterward (Gin/Echo behavior).
func (c *Context) Status(code int) *Context {
	if !c.headerWritten {
		c.status = code
	}
	return c
}

// FlushHeaders writes a pending Status() when the handler sent no body.
func (c *Context) FlushHeaders() {
	if !c.headerWritten && c.status != 0 {
		c.writeHeader(c.status)
	}
}

func (c *Context) writeHeader(code int) {
	if c.headerWritten {
		return
	}
	if code == 0 {
		if c.status != 0 {
			code = c.status
		} else {
			code = http.StatusOK
		}
	}
	c.status = code
	c.Writer.WriteHeader(code)
	c.headerWritten = true
}

// String sends a plain text response. Extra args are fmt.Sprintf'd into format.
func (c *Context) String(code int, format string, values ...interface{}) error {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.writeHeader(code)
	if len(values) > 0 {
		format = fmt.Sprintf(format, values...)
	}
	_, err := c.Writer.Write([]byte(format))
	return err
}

// RenderHTML renders the template with data and sets values content-type to "text/html".
func (c *Context) RenderHTML(code int, name string, data interface{}) error {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if c.Templates == nil {
		return c.String(http.StatusInternalServerError, "Templates not loaded")
	}
	c.writeHeader(code)
	return c.Templates.ExecuteTemplate(c.Writer, name, data)
}

// HTML sends an HTML response (Raw String).
func (c *Context) HTML(code int, html string) error {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.writeHeader(code)
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

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     sameOrigin,
}

// SetWebsocketCheckOrigin sets the WebSocket origin check.
// Pass nil to restore same-origin (production default).
func SetWebsocketCheckOrigin(fn func(*http.Request) bool) {
	if fn == nil {
		upgrader.CheckOrigin = sameOrigin
		return
	}
	upgrader.CheckOrigin = fn
}

// Upgrade upgrades the HTTP connection to a WebSocket connection.
// Returns the *websocket.Conn and any error.
func (c *Context) Upgrade() (*websocket.Conn, error) {
	return upgrader.Upgrade(c.Writer, c.Request, nil)
}
