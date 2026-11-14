package context

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/go-kvolt/kvolt/v2/router"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
)

const rawBodyKey = "_kvolt_raw_body"

// absPath / pathRel / evalSymlinks are overridable in tests for hard FS error paths.
var absPath = filepath.Abs
var pathRel = filepath.Rel
var evalSymlinks = filepath.EvalSymlinks

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

	// TrustedProxies is set by the Engine for ClientIP (CIDRs or IPs).
	TrustedProxies []string

	// RoutePath is an optional metrics/debug label set by the Engine.
	RoutePath string

	aborted  bool
	detached bool
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

// SetHeaderWritten marks that response headers have been written.
func (c *Context) SetHeaderWritten() {
	c.headerWritten = true
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
	c.TrustedProxies = nil
	c.RoutePath = ""
	c.index = -1
	c.status = 0
	c.headerWritten = false
	c.aborted = false
	c.detached = false
}

// SetDetached marks the context as detached, signaling that an async goroutine
// may outlive the main handler and this context instance must not be returned to sync.Pool.
func (c *Context) SetDetached() {
	c.detached = true
}

// IsDetached reports whether the context is detached.
func (c *Context) IsDetached() bool {
	return c.detached
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
// httperr.Error is mapped to JSON; other errors become a generic 500.
func (c *Context) Next() {
	if c.aborted {
		return
	}
	c.index++
	if c.index < len(c.Handlers) {
		handler := c.Handlers[c.index]
		if err := handler(c); err != nil {
			c.WriteHandlerError(err)
			return
		}
	}
}

// Abort stops the remaining middleware/handler chain.
func (c *Context) Abort() {
	c.aborted = true
	c.index = len(c.Handlers)
}

// IsAborted reports whether Abort was called.
func (c *Context) IsAborted() bool {
	return c.aborted
}

// WriteHandlerError maps handler errors to JSON responses when headers are not written yet.
func (c *Context) WriteHandlerError(err error) {
	if c.headerWritten {
		log.Printf("[KVolt] handler error after write: %v", err)
		return
	}
	if he, ok := httperr.As(err); ok {
		_ = c.JSON(he.Status, he)
		return
	}
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) || strings.Contains(err.Error(), "http: request body too large") {
		_ = c.JSON(http.StatusRequestEntityTooLarge, httperr.PayloadTooLarge(""))
		return
	}
	log.Printf("[KVolt] handler error: %v", err)
	c.writeJSONBytes(http.StatusInternalServerError, errInternalJSON)
}

// AbortWithError stops the chain and writes an httperr JSON response (no double-write log).
func (c *Context) AbortWithError(err *httperr.Error) error {
	c.Abort()
	if err == nil {
		return nil
	}
	if !c.headerWritten {
		_ = c.JSON(err.Status, err)
	}
	return nil
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

// Data writes raw bytes with the specified status code and optional content type.
func (c *Context) Data(code int, contentType string, data []byte) error {
	if contentType != "" {
		c.Writer.Header().Set("Content-Type", contentType)
	}
	c.writeHeader(code)
	_, err := c.Writer.Write(data)
	return err
}

// RawBody reads the full request body once and restores Request.Body so Bind/BindJSON still work.
// Useful for webhook signature verification (Razorpay, Stripe, couriers).
func (c *Context) RawBody() ([]byte, error) {
	if v, ok := c.Get(rawBodyKey); ok {
		if b, ok := v.([]byte); ok {
			return b, nil
		}
	}
	if c.Request == nil || c.Request.Body == nil {
		return nil, io.EOF
	}
	b, err := io.ReadAll(c.Request.Body)
	_ = c.Request.Body.Close()
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(b))
	c.Set(rawBodyKey, b)
	return b, nil
}

// FormFile returns the first file for the provided form key (32MB memory max for multipart).
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	return c.FormFileMax(name, 32<<20)
}

// FormFileMax is like FormFile but lets you set ParseMultipartForm maxMemory.
func (c *Context) FormFileMax(name string, maxMemory int64) (*multipart.FileHeader, error) {
	if maxMemory <= 0 {
		maxMemory = 32 << 20
	}
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(maxMemory); err != nil {
			return nil, err
		}
	}
	f, fh, err := c.Request.FormFile(name)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	return fh, nil
}

// FormFileAllowed returns the file only when its extension is in allowedExts (e.g. ".jpg", ".png").
// Comparison is case-insensitive. Empty allowedExts means any extension.
func (c *Context) FormFileAllowed(name string, maxMemory int64, allowedExts ...string) (*multipart.FileHeader, error) {
	fh, err := c.FormFileMax(name, maxMemory)
	if err != nil {
		return nil, err
	}
	if len(allowedExts) == 0 {
		return fh, nil
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	for _, a := range allowedExts {
		if ext == strings.ToLower(a) {
			return fh, nil
		}
	}
	return nil, httperr.BadRequest("invalid_file_type", "file extension not allowed")
}

// SaveUploadedFile uploads the form file to dst.
// dst must be a local path (filepath.IsLocal); ".." escapes are rejected.
// Prefer SafeUploadPath(dir, file.Filename) when using the client-provided name.
func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	dst = filepath.Clean(dst)
	if !filepath.IsAbs(dst) && (dst == "." || !filepath.IsLocal(dst)) {
		return httperr.BadRequest("invalid_path", "upload destination must be a local non-escaping path")
	}
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

// SafeUploadPath joins dir with the base name of filename (blocks path traversal).
func SafeUploadPath(dir, filename string) string {
	base := filepath.Base(filename)
	if base == "." || base == ".." || base == "" {
		base = "upload"
	}
	return filepath.Join(dir, base)
}

// File writes the specified file into the body stream in an efficient way.
// Do not pass user-controlled paths; use FileFrom when the name comes from the request.
func (c *Context) File(path string) {
	http.ServeFile(c.Writer, c.Request, path)
	c.headerWritten = true
	if c.status == 0 {
		c.status = http.StatusOK
	}
}

// FileFrom serves a file under root. name may not escape root (path traversal blocked).
func (c *Context) FileFrom(root, name string) error {
	target, err := safeJoinRoot(root, name)
	if err != nil {
		return httperr.BadRequest("invalid_path", "file path escapes root")
	}
	c.File(target)
	return nil
}

// Attachment serves a file as a download with Content-Disposition attachment.
func (c *Context) Attachment(path, filename string) {
	if filename == "" {
		filename = filepath.Base(path)
	}
	filename = sanitizeDownloadFilename(filename)
	c.Writer.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.File(path)
}

// AttachmentFrom serves a download under root (blocks path traversal).
func (c *Context) AttachmentFrom(root, name, filename string) error {
	target, err := safeJoinRoot(root, name)
	if err != nil {
		return httperr.BadRequest("invalid_path", "file path escapes root")
	}
	if filename == "" {
		filename = filepath.Base(target)
	}
	c.Attachment(target, filename)
	return nil
}

func safeJoinRoot(root, name string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", httperr.BadRequest("invalid_path", "empty root")
	}
	root = filepath.Clean(root)
	absRoot, err := absPath(root)
	if err != nil {
		return "", httperr.BadRequest("invalid_path", "invalid root")
	}
	if resolved, err := evalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}

	target := filepath.Clean(filepath.Join(absRoot, name))
	if !underRoot(absRoot, target) {
		return "", httperr.BadRequest("invalid_path", "path escapes root")
	}
	// If the path exists (file, dir, or symlink), require successful symlink resolution
	// so a link inside root cannot point outside. Missing paths stay lexical-only (404 later).
	if _, err := os.Lstat(target); err == nil {
		resolved, err := evalSymlinks(target)
		if err != nil {
			return "", httperr.BadRequest("invalid_path", "path escapes root")
		}
		if !underRoot(absRoot, resolved) {
			return "", httperr.BadRequest("invalid_path", "path escapes root")
		}
		return resolved, nil
	}
	return target, nil
}

func underRoot(root, target string) bool {
	rel, err := pathRel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func sanitizeDownloadFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		if r < 32 || r == '"' || r == '\\' || r == '\n' || r == '\r' || r == ';' {
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "download"
	}
	return out
}

// Redirect sends an HTTP redirect to a safe relative location only.
// Absolute URLs (http/https/javascript/…) and protocol-relative "//…" are rejected
// to prevent open redirects. Use RedirectURL for trusted absolute URLs.
func (c *Context) Redirect(code int, location string) error {
	if !isSafeRelativeRedirect(location) {
		return httperr.BadRequest("unsafe_redirect", "only relative redirects are allowed")
	}
	if code < 300 || code > 308 {
		code = http.StatusFound
	}
	http.Redirect(c.Writer, c.Request, location, code)
	c.headerWritten = true
	c.status = code
	return nil
}

// RedirectURL redirects to any URL. Use only with trusted/absolute locations you control.
func (c *Context) RedirectURL(code int, location string) error {
	if strings.TrimSpace(location) == "" {
		return httperr.BadRequest("unsafe_redirect", "empty redirect location")
	}
	if strings.ContainsAny(location, "\r\n\x00") {
		return httperr.BadRequest("unsafe_redirect", "invalid characters in redirect location")
	}
	if code < 300 || code > 308 {
		code = http.StatusFound
	}
	http.Redirect(c.Writer, c.Request, location, code)
	c.headerWritten = true
	c.status = code
	return nil
}

func isSafeRelativeRedirect(location string) bool {
	location = strings.TrimSpace(location)
	if location == "" {
		return false
	}
	// Only same-origin path redirects (leading single slash).
	if !strings.HasPrefix(location, "/") || strings.HasPrefix(location, "//") {
		return false
	}
	// Backslash / CRLF / NUL / @ (userinfo) tricks historically confuse some user-agents.
	if strings.ContainsAny(location, "\\\r\n\x00@") {
		return false
	}
	if strings.Contains(location, "://") {
		return false
	}
	lower := strings.ToLower(location)
	if strings.Contains(lower, "javascript:") || strings.Contains(lower, "data:") || strings.Contains(lower, "vbscript:") {
		return false
	}
	return true
}

// NoContent sends 204 No Content.
func (c *Context) NoContent() error {
	c.writeHeader(http.StatusNoContent)
	return nil
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
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err == nil {
		c.headerWritten = true
		c.status = http.StatusSwitchingProtocols
	}
	return conn, err
}
