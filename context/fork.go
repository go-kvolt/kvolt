package context

import (
	"net/http"

	"github.com/go-kvolt/kvolt/v2/router"
)

// Fork returns an independent copy of c that writes to w and reads from r, positioned
// at the same point in the handler chain.
//
// It is for middleware that runs the rest of the chain on another goroutine and may
// stop waiting for it (middleware.Timeout). A Context is not safe for concurrent use
// and the Engine reuses it for later requests, so a goroutine that can outlive the
// request must never hold the original: it gets the fork instead. The fork shares
// nothing mutable with c — route params and Keys are copied — and is never pooled.
//
// Call fork.Next() to run the remaining handlers, and c.Join(fork) once they have
// finished to carry their outcome back. If they have not finished, do not Join: leave
// the fork to the goroutine that is still using it.
func (c *Context) Fork(w http.ResponseWriter, r *http.Request) *Context {
	if c == nil {
		return New(w, r)
	}
	f := &Context{
		Writer:         w,
		Request:        r,
		Handlers:       c.Handlers,
		Params:         append(router.Params(nil), c.Params...),
		index:          c.index,
		status:         c.status,
		headerWritten:  c.headerWritten,
		Templates:      c.Templates,
		TrustedProxies: c.TrustedProxies,
		RoutePath:      c.RoutePath,
		aborted:        c.aborted,
	}
	if len(c.Keys) > 0 {
		f.Keys = make(map[string]interface{}, len(c.Keys))
		for k, v := range c.Keys {
			f.Keys[k] = v
		}
	}
	return f
}

// Join copies the outcome of a finished fork back into c: the response status, whether
// headers were written, the abort and detached flags, and the values stored with Set.
// The fork must no longer be in use by any goroutine.
func (c *Context) Join(f *Context) {
	if c == nil || f == nil {
		return
	}
	c.status = f.status
	c.headerWritten = f.headerWritten
	c.aborted = f.aborted
	if f.detached {
		c.detached = true
	}
	if f.Keys != nil {
		c.Keys = f.Keys
	}
}
