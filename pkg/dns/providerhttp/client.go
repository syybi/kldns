package providerhttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const DefaultHTTPTimeout = 30 * time.Second

// ProxyURLKey is the shared provider config key for an optional HTTP/SOCKS5 proxy.
const ProxyURLKey = "ProxyURL"

func NewClient() *http.Client {
	client, _ := NewClientWithProxy("")
	return client
}

// NewClientWithProxy builds an HTTP client. proxyURL may be empty, or an
// http(s):// / socks5:// / socks5h:// URL. All requests made with the client
// use that proxy when set.
func NewClientWithProxy(proxyURL string) (*http.Client, error) {
	transport, err := newTransport(proxyURL)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout:   DefaultHTTPTimeout,
		Transport: transport,
	}, nil
}

func newTransport(proxyURL string) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return transport, nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid proxy url: missing host")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		transport.Proxy = http.ProxyURL(u)
		return transport, nil
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(u, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("invalid socks5 proxy: %w", err)
		}
		if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
		// SOCKS dialer handles connectivity; do not also apply an HTTP proxy.
		transport.Proxy = nil
		return transport, nil
	case "":
		return nil, fmt.Errorf("invalid proxy url: missing scheme (use http:// or socks5://)")
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q (use http or socks5)", u.Scheme)
	}
}

func NormalizeBaseURL(raw string, fallback string, trailingSlash bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = fallback
	}
	raw = strings.TrimRight(raw, "/")
	if trailingSlash {
		return raw + "/"
	}
	return raw
}

func JoinBaseURL(baseURL string, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}
