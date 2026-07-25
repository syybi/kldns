package providerhttp

import (
	"net/http"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		fallback      string
		trailingSlash bool
		want          string
	}{
		{name: "fallback without slash", fallback: "https://api.example.com/", want: "https://api.example.com"},
		{name: "fallback with slash", fallback: "https://api.example.com", trailingSlash: true, want: "https://api.example.com/"},
		{name: "raw trims spaces", raw: " https://mock.example.com/// ", fallback: "https://api.example.com", want: "https://mock.example.com"},
		{name: "raw with trailing slash", raw: "https://mock.example.com", fallback: "https://api.example.com", trailingSlash: true, want: "https://mock.example.com/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeBaseURL(tt.raw, tt.fallback, tt.trailingSlash)
			if got != tt.want {
				t.Fatalf("NormalizeBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewClientUsesDefaultTimeout(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.Timeout != DefaultHTTPTimeout {
		t.Fatalf("Timeout = %s, want %s", client.Timeout, DefaultHTTPTimeout)
	}
	if _, ok := any(client).(*http.Client); !ok {
		t.Fatal("NewClient should return *http.Client")
	}
}

func TestNewClientWithProxyAcceptsHTTPAndSOCKS5(t *testing.T) {
	for _, proxyURL := range []string{
		"",
		"http://127.0.0.1:7890",
		"https://user:pass@proxy.example:8443",
		"socks5://127.0.0.1:1080",
		"socks5h://user:pass@127.0.0.1:1080",
	} {
		client, err := NewClientWithProxy(proxyURL)
		if err != nil {
			t.Fatalf("proxy %q: %v", proxyURL, err)
		}
		if client == nil || client.Transport == nil {
			t.Fatalf("proxy %q returned empty client", proxyURL)
		}
	}
}

func TestNewClientWithProxyRejectsInvalid(t *testing.T) {
	cases := []string{
		"://bad",
		"ftp://127.0.0.1:21",
		"not-a-url",
		"http://",
	}
	for _, proxyURL := range cases {
		if _, err := NewClientWithProxy(proxyURL); err == nil {
			t.Fatalf("proxy %q should fail", proxyURL)
		}
	}
}
