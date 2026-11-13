package context

import (
	"net/http"
	"time"
)

// CookieOptions configures SetCookieWith.
type CookieOptions struct {
	Path     string
	Domain   string
	MaxAge   int
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
}

// SetCookie sets a cookie on the response.
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	c.SetCookieWith(name, value, CookieOptions{
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HTTPOnly: httpOnly,
		SameSite: http.SameSiteLaxMode,
	})
}

// SetCookieWith sets a cookie with full options (including SameSite).
func (c *Context) SetCookieWith(name, value string, opt CookieOptions) {
	if opt.Path == "" {
		opt.Path = "/"
	}
	// #nosec G124 -- cookie security flags are configurable via CookieOptions
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     opt.Path,
		Domain:   opt.Domain,
		MaxAge:   opt.MaxAge,
		Secure:   opt.Secure,
		HttpOnly: opt.HTTPOnly,
		SameSite: opt.SameSite,
	}
	if opt.MaxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(opt.MaxAge) * time.Second)
	}
	http.SetCookie(c.Writer, cookie)
}

// ClearCookie expires a cookie on the client.
func (c *Context) ClearCookie(name, path, domain string) {
	c.ClearCookieWith(name, CookieOptions{Path: path, Domain: domain, HTTPOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

// ClearCookieWith expires a cookie using the same flags that were used when setting it.
func (c *Context) ClearCookieWith(name string, opt CookieOptions) {
	if opt.Path == "" {
		opt.Path = "/"
	}
	// #nosec G124 -- clearing cookie uses caller-specified flags
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     opt.Path,
		Domain:   opt.Domain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: opt.HTTPOnly,
		Secure:   opt.Secure,
		SameSite: opt.SameSite,
	})
}
