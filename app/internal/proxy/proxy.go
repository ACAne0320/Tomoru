// Package proxy is how the app reaches the cloud, as the user chose in Preferences: through the
// proxy the system sets, as browsers do (MyGo's netproxy, with its bypass list and PAC files),
// through one of the user's own, or directly. Loopback, where the local models' servers are, is
// always reached directly.
//
// Install puts it in Go's default transport, which the app's clients use: the cloud's recognition
// and translation, the user's APIs, model downloads.
package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo/netproxy"
)

type choice struct {
	off bool
	url *url.URL // the user's proxy; nil for the system's
}

var current atomic.Pointer[choice]

// Set chooses how the cloud is reached from now on: with off, directly; else through addr, or the
// system's proxy when addr is "" or no address. Requests under way keep their connections.
func Set(off bool, addr string) {
	u, _ := Parse(addr)
	current.Store(&choice{off: off, url: u})
}

// Parse reads a proxy's address, "host:port" or a URL of http, https or socks5: nil for "".
func Parse(addr string) (*url.URL, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, nil
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil || u.Hostname() == "" || u.Port() == "" || u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("proxy: not an address: %q", addr)
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("proxy: not a proxy's scheme: %q", u.Scheme)
	}
	u.Path = ""
	return u, nil
}

// For is the proxy of req, or nil to connect directly: an http.Transport's Proxy, safe from any
// goroutine. The system's may block while the system runs a PAC file.
func For(req *http.Request) (*url.URL, error) {
	c := current.Load()
	switch {
	case loopback(req.URL.Hostname()):
		return nil, nil
	case c != nil && c.off:
		return nil, nil
	case c != nil && c.url != nil:
		return c.url, nil
	}
	return netproxy.Proxy(req)
}

// Install has Go's default transport ask For.
func Install() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = For
	}
}

// System is the proxy the system sets for target, a URL, as "host:port" (or a URL, for another
// scheme than http): "" for none. It may block while the system runs a PAC file.
func System(target string) string {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return ""
	}
	u, err := netproxy.Proxy(req)
	if err != nil || u == nil {
		return ""
	}
	if u.Scheme == "http" {
		return u.Host
	}
	return u.Scheme + "://" + u.Host
}

func loopback(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
