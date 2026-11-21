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
	url *url.URL
}

var current atomic.Pointer[choice]

func Set(off bool, addr string) {
	u, _ := Parse(addr)
	current.Store(&choice{off: off, url: u})
}

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

func Install() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = For
	}
}

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
