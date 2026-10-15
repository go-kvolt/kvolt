package kvolt

import (
	"net"
	"strings"
)

func isTrustedProxy(remoteAddr string, trusted []string) bool {
	if len(trusted) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
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
