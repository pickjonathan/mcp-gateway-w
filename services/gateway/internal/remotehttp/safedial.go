package remotehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// blockedEgressError is returned when a downstream address resolves to a
// non-public IP (SSRF protection).
type blockedEgressError struct{ addr string }

func (e *blockedEgressError) Error() string {
	return fmt.Sprintf("remotehttp: blocked egress to non-public address %s", e.addr)
}

// isBlockedIP reports whether ip is in a range an untrusted, admin-supplied
// endpoint must not reach: loopback, private (RFC1918 / IPv6 ULA), link-local
// (incl. the cloud metadata service 169.254.169.254 and fe80::/10), multicast,
// or unspecified. Public addresses are allowed.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// guardedDialContext resolves the target host and connects only to a public IP,
// rejecting internal ranges. The check runs at dial time against the actual IP,
// so it also defeats DNS rebinding (a hostname that resolves public at
// registration but internal at call time).
func guardedDialContext(base *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			if isBlockedIP(ip) {
				lastErr = &blockedEgressError{addr: ip.String()}
				continue
			}
			conn, derr := base.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if derr != nil {
				lastErr = derr
				continue
			}
			return conn, nil
		}
		if lastErr == nil {
			lastErr = &blockedEgressError{addr: host}
		}
		return nil, lastErr
	}
}

// guardedTransport returns an http.Transport whose dialer blocks internal IPs.
func guardedTransport() *http.Transport {
	base := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		DialContext:           guardedDialContext(base),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}
