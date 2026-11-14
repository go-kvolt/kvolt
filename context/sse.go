package context

import (
	"fmt"
	"net/http"
	"strings"
)

// SSEvent is a Server-Sent Event payload.
type SSEvent struct {
	Event string
	Data  string
	ID    string
	Retry int // milliseconds; 0 omits the field
}

// SSE prepares the response for Server-Sent Events and writes headers.
func (c *Context) SSE() error {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	if c.status == 0 {
		c.status = http.StatusOK
	}
	if !c.headerWritten {
		c.Writer.WriteHeader(c.status)
		c.headerWritten = true
	}
	return c.Flush()
}

// Event writes a simple SSE frame (event + data).
func (c *Context) Event(event, data string) error {
	return c.SSEvent(SSEvent{Event: event, Data: data})
}

// SSEvent writes a full SSE frame.
// Event and ID are sanitized (CR/LF stripped) so untrusted values cannot inject frames.
func (c *Context) SSEvent(ev SSEvent) error {
	if !c.headerWritten {
		if err := c.SSE(); err != nil {
			return err
		}
	}
	var b strings.Builder
	if id := sanitizeSSEField(ev.ID); id != "" {
		b.WriteString("id: ")
		b.WriteString(id)
		b.WriteByte('\n')
	}
	if event := sanitizeSSEField(ev.Event); event != "" {
		b.WriteString("event: ")
		b.WriteString(event)
		b.WriteByte('\n')
	}
	if ev.Retry > 0 {
		fmt.Fprintf(&b, "retry: %d\n", ev.Retry)
	}
	data := strings.ReplaceAll(ev.Data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	for _, line := range strings.Split(data, "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	if _, err := c.Writer.Write([]byte(b.String())); err != nil {
		return err
	}
	c.headerWritten = true
	return c.Flush()
}

func sanitizeSSEField(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// Flush flushes buffered data to the client when the writer supports http.Flusher.
func (c *Context) Flush() error {
	f, ok := c.Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("sse: ResponseWriter does not support Flush")
	}
	f.Flush()
	return nil
}
