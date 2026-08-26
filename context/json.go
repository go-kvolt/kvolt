package context

import (
	"encoding/json"
	"sync"

	"github.com/bytedance/sonic/encoder"
)

const contentTypeJSON = "application/json; charset=utf-8"

var errInternalJSON = []byte(`{"error":"Internal Server Error"}`)

const jsonEncodeOpts = encoder.NoValidateJSONMarshaler | encoder.NoEncoderNewline

var jsonOutPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 256)
		return &b
	},
}

// BindJSON decodes the JSON body into obj. It does not run struct validation.
// encoding/json is used here so tiny API bodies (invoice, ping-sized) stay
// allocation-light and match net/http apps; sonic still encodes responses.
func (c *Context) BindJSON(obj interface{}) error {
	return json.NewDecoder(c.Request.Body).Decode(obj)
}

// JSON sends a JSON response with sonic's fastest encoder into a pooled buffer.
func (c *Context) JSON(code int, obj interface{}) error {
	c.Writer.Header().Set("Content-Type", contentTypeJSON)
	c.writeHeader(code)
	bp := jsonOutPool.Get().(*[]byte)
	buf := (*bp)[:0]
	err := encoder.EncodeInto(&buf, obj, jsonEncodeOpts)
	if err == nil {
		_, err = c.Writer.Write(buf)
	}
	*bp = buf
	jsonOutPool.Put(bp)
	return err
}

func (c *Context) writeJSONBytes(code int, body []byte) {
	c.Writer.Header().Set("Content-Type", contentTypeJSON)
	c.writeHeader(code)
	_, _ = c.Writer.Write(body)
}

// InternalError writes a generic 500 JSON body if headers are not already sent.
func (c *Context) InternalError() {
	if !c.headerWritten {
		c.writeJSONBytes(500, errInternalJSON)
	}
}
