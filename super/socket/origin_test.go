package socket

import (
	"net/http/httptest"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/constants"
)

func TestCheckOrigin(t *testing.T) {
	cases := []struct {
		name         string
		appURL, port string
		host, origin string
		want         bool
	}{
		{"local dev: APP_URL + APP_PORT", "https://localhost", "8443", "localhost:8443", "https://localhost:8443", true},
		{"behind proxy: same origin, no port", "https://chat.example.com", "8080", "chat.example.com", "https://chat.example.com", true},
		{"behind proxy: APP_URL without port", "https://chat.example.com", "8080", "127.0.0.1:8080", "https://chat.example.com", true},
		{"APP_URL with trailing slash", "https://chat.example.com/", "8080", "127.0.0.1:8080", "https://chat.example.com", true},
		{"same origin, host case differs", "", "8080", "Chat.Example.com", "https://chat.example.com", true},
		{"LAN name not in APP_URL but same origin", "https://localhost", "8443", "mac.local:8443", "https://mac.local:8443", true},
		{"other site", "https://chat.example.com", "8080", "chat.example.com", "https://evil.example", false},
		{"other port on same host name", "https://chat.example.com", "8080", "chat.example.com", "https://chat.example.com:9999", false},
		{"lookalike host", "https://chat.example.com", "8080", "chat.example.com", "https://chat.example.com.evil.example", false},
		{"no Origin header", "https://chat.example.com", "8080", "chat.example.com", "", false},
		{"null origin", "https://chat.example.com", "8080", "chat.example.com", "null", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(constants.APP_URL, c.appURL)
			t.Setenv(constants.APP_PORT, c.port)
			r := httptest.NewRequest("GET", "/ws", nil)
			r.Host = c.host
			if c.origin != "" {
				r.Header.Set("Origin", c.origin)
			}
			if got := checkOrigin(r); got != c.want {
				t.Fatalf("checkOrigin(Origin=%q, Host=%q) = %v, want %v", c.origin, c.host, got, c.want)
			}
		})
	}
}
