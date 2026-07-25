package dns

import (
	"errors"
	"net/http"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unrelated", err: errors.New("timeout"), want: false},
		{name: "auth notfound code", err: &ProviderError{Message: "InvalidAccessKeyId.NotFound"}, want: false},
		{name: "flag", err: &ProviderError{NotFound: true, Message: "gone"}, want: true},
		{name: "status 404", err: &ProviderError{StatusCode: http.StatusNotFound, Message: "missing"}, want: true},
		{name: "record not found", err: &ProviderError{Message: "record not found"}, want: true},
		{name: "record value not found", err: &ProviderError{Message: "record value not found"}, want: true},
		{name: "http 404 text", err: &ProviderError{Message: "Cloudflare API returned HTTP 404"}, want: true},
		{name: "chinese", err: &ProviderError{Message: "记录不存在"}, want: true},
		{name: "aliyun domain record", err: &ProviderError{Message: "The specified domain record does not exist."}, want: true},
		{name: "wrapped", err: errors.Join(errors.New("outer"), &ProviderError{Message: "record not found"}), want: true},
		{name: "plain string", err: errors.New("dnspod delete_record: 记录不存在"), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNotFound(tc.err); got != tc.want {
				t.Fatalf("IsNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
