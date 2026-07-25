package providers

import (
	"net/http"
	"strings"

	"kldns/pkg/dns"
	"kldns/pkg/dns/providerhttp"
)

func proxyConfigField() dns.ConfigField {
	return dns.ConfigField{
		Name:        providerhttp.ProxyURLKey,
		Label:       "代理地址",
		Description: "可选。此配置下所有 API 请求走该代理，支持 http:// 与 socks5://，例如 http://127.0.0.1:7890 或 socks5://user:pass@127.0.0.1:1080",
	}
}

func withProxyField(fields ...dns.ConfigField) []dns.ConfigField {
	return append(fields, proxyConfigField())
}

// applyHTTPClient sets provider HTTP client from config.
// When ProxyURL is set, a new proxied client is always built.
// When ProxyURL is empty, an existing injected client is kept (tests), otherwise a default client is used.
func applyHTTPClient(current **http.Client, config map[string]string) error {
	proxyURL := strings.TrimSpace(config[providerhttp.ProxyURLKey])
	if proxyURL != "" {
		client, err := providerhttp.NewClientWithProxy(proxyURL)
		if err != nil {
			return err
		}
		*current = client
		return nil
	}
	if *current == nil {
		*current = providerhttp.NewClient()
	}
	return nil
}
