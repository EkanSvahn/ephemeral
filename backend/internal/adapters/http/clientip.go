package http

import (
	"net"
	"net/http"
	"strings"
)

// newClientIPResolver returns a function that picks the address a request is
// attributed to, for rate limiting and for the access log.
//
// X-Forwarded-For is only read when the direct peer is inside
// trustedProxyCIDR. It is then read from the right: the last entry was added
// by the trusted proxy, and everything to the left of it was supplied by
// whoever connected to that proxy. The first address from the right that is
// not itself a trusted proxy is the client. Reading from the left would let a
// client choose its own address whenever a proxy appends to the header
// instead of replacing it.
func newClientIPResolver(trustedProxyCIDR string) func(*http.Request) string {
	_, trustedNet, err := net.ParseCIDR(strings.TrimSpace(trustedProxyCIDR))
	if err != nil {
		trustedNet = nil
	}

	return func(r *http.Request) string {
		direct := remoteIP(r)
		if trustedNet == nil {
			return direct
		}
		parsedDirect := net.ParseIP(direct)
		if parsedDirect == nil || !trustedNet.Contains(parsedDirect) {
			return direct
		}

		// A request can carry the header more than once. The lines form one
		// list, in the order they were sent.
		parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(parts[i])
			if candidate == "" {
				continue
			}
			ip := net.ParseIP(candidate)
			if ip == nil {
				// Nothing to the left of a malformed entry can be trusted.
				return direct
			}
			if !trustedNet.Contains(ip) {
				return ip.String()
			}
		}
		return direct
	}
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil || host == "" {
		if r.RemoteAddr == "" {
			return "unknown"
		}
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

// rateLimitClientKey maps a client address to the key its requests are counted
// under. IPv4 addresses are counted one by one. IPv6 addresses are counted per
// /64, because a single subscriber normally controls a whole /64 and could
// otherwise get a fresh rate limit for every request.
func rateLimitClientKey(clientIP string) string {
	ip := net.ParseIP(clientIP)
	if ip == nil || ip.To4() != nil {
		return clientIP
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}
