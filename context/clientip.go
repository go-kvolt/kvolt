package context

import (
	"net"
	"strings"
)

// ClientIP returns the client IP.
// When the peer is in TrustedProxies, X-Forwarded-For is walked right-to-left
// (skipping trusted hops) so a client-spoofed left-most value is ignored when
// proxies append. Falls back to X-Real-IP, then the peer address.
func (c *Context) ClientIP() string {
	if c.Request == nil {
		return ""
	}
	remote := c.Request.RemoteAddr
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	if !isTrustedProxy(host, c.TrustedProxies) {
		return host
	}
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := clientIPFromXFF(xff, c.TrustedProxies); ip != "" {
			return ip
		}
	}
	// X-Real-IP is only trusted when it itself is not a TrustedProxies hop
	// (proxies should put the real client there). Never return a trusted proxy
	// address as the client IP — fall back to the peer.
	if xri := strings.TrimSpace(c.Request.Header.Get("X-Real-IP")); xri != "" {
		if h, _, err := net.SplitHostPort(xri); err == nil {
			xri = h
		}
		if !isTrustedProxy(xri, c.TrustedProxies) {
			return xri
		}
	}
	return host
}

// clientIPFromXFF picks the rightmost untrusted address in an X-Forwarded-For list.
func clientIPFromXFF(xff string, trusted []string) string {
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if h, _, err := net.SplitHostPort(ip); err == nil {
			ip = h
		}
		if isTrustedProxy(ip, trusted) {
			continue
		}
		return ip
	}
	return ""
}

func isTrustedProxy(host string, trusted []string) bool {
	if len(trusted) == 0 {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, t := range trusted {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if strings.Contains(t, "/") {
			_, cidr, err := net.ParseCIDR(t)
			if err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if tip := net.ParseIP(t); tip != nil && tip.Equal(ip) {
			return true
		}
	}
	return false
}
