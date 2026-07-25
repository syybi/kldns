package dns

import (
	"errors"
	"net/http"
	"strings"
)

type ProviderError struct {
	Provider   string
	Operation  string
	Message    string
	StatusCode int
	NotFound   bool
	Cause      error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Provider + " " + e.Operation + ": " + e.Message
	}
	return e.Provider + " " + e.Operation + ": " + e.Message + ": " + e.Cause.Error()
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// IsNotFound reports whether err means the remote DNS record (or value) no longer exists.
// Delete flows treat this as success so local cleanup can continue.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var pe *ProviderError
	if errors.As(err, &pe) {
		if pe.NotFound || pe.StatusCode == http.StatusNotFound {
			return true
		}
		if isNotFoundMessage(pe.Message) {
			return true
		}
	}
	return isNotFoundMessage(err.Error())
}

func isNotFoundMessage(message string) bool {
	message = strings.TrimSpace(message)
	if message == "" {
		return false
	}
	lower := strings.ToLower(message)

	// Prefer precise record-not-found signals over bare "not found", which can
	// appear in unrelated auth/config errors (e.g. InvalidAccessKeyId.NotFound).
	exactSignals := []string{
		"record not found",
		"record value not found",
		"dns record not found",
		"record does not exist",
		"records does not exist",
		"no such record",
		"no such host record",
		"invaliddomainrecord.notfound",
		"nosuchdnsrecord",
		"recordnotfound",
		"resource not found",
		"returned http 404",
		"http 404",
		"记录不存在",
		"解析记录不存在",
		"域名记录不存在",
		"记录id不存在",
		"记录 id 不存在",
		"找不到记录",
		"无法找到记录",
	}
	for _, signal := range exactSignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	// Generic 404 phrasing used by some providers when message body is empty.
	if strings.Contains(lower, "404") && (strings.Contains(lower, "not found") || strings.Contains(lower, "不存在")) {
		return true
	}
	return false
}
