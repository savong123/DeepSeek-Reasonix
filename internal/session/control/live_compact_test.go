package control

import (
	"testing"

	"reasonix/internal/contract/config"
)

func TestLiveCompactEndpointSelection(t *testing.T) {
	for _, tc := range []struct {
		name, base, request, chat string
		cloud                     bool
	}{
		{"local override", "https://public.example/v1", "http://[::1]:8222/chat", "https://other.example/chat", false},
		{"exact cloud override", "http://localhost/v1", "https://public.example/chat", "http://localhost/chat", true},
		{"same-base request falls back to chat", "https://public.example/v1", "https://public.example/v1/", "http://10.0.0.1/chat", false},
		{"private", "http://10.0.0.1/chat", "", "", false},
		{"unknown", "", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := &config.ProviderEntry{Kind: "openai", BaseURL: tc.base, RequestURL: tc.request, ChatURL: tc.chat}
			if got := liveCompactCloud(entry); got != tc.cloud {
				t.Fatalf("cloud=%t, want %t", got, tc.cloud)
			}
		})
	}
}
