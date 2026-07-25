package providers

import (
	"context"
	"testing"

	"kldns/pkg/dns"
)

func TestAllLegacyProvidersAreRegistered(t *testing.T) {
	want := []string{
		"Dnspod", "Aliyun", "DnsCom", "DnsLa", "DnsDun", "West",
		"HuaweiCloud", "BaiduCloud", "Route53", "GoogleCloudDns", "Cloudflare",
	}
	for _, key := range want {
		t.Run(key, func(t *testing.T) {
			provider, ok := dns.New(key)
			if !ok {
				t.Fatalf("provider %s is not registered", key)
			}
			if provider.Label() == "" || len(provider.ConfigFields()) == 0 {
				t.Fatalf("provider %s has incomplete metadata", key)
			}
			if !hasConfigField(provider.ConfigFields(), "ProxyURL") {
				t.Fatalf("provider %s missing ProxyURL config field", key)
			}
		})
	}
}

func TestProviderConfigureRejectsInvalidProxy(t *testing.T) {
	provider, ok := dns.New("Cloudflare")
	if !ok {
		t.Fatal("Cloudflare provider not registered")
	}
	err := provider.Configure(map[string]string{
		"ApiToken": "token",
		"ProxyURL": "ftp://127.0.0.1:21",
	})
	if err == nil {
		t.Fatal("expected invalid proxy error")
	}
}

func hasConfigField(fields []dns.ConfigField, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}
	return false
}

func TestProviderCheckRequiresConfiguredSecrets(t *testing.T) {
	provider, ok := dns.New("GoogleCloudDns")
	if !ok {
		t.Fatal("GoogleCloudDns provider not registered")
	}
	if err := provider.Check(context.Background()); err == nil {
		t.Fatal("expected missing config error")
	}
}
