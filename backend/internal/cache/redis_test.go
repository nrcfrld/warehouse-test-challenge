package cache

import (
	"testing"
	"time"
)

func TestConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, url string
		ttl       time.Duration
		ok        bool
	}{
		{"valid", "redis://localhost:6379/0", time.Minute, true},
		{"tls", "rediss://localhost:6379/0", time.Second, true},
		{"invalid URL", "http://localhost:6379", time.Minute, false},
		{"zero TTL", "redis://localhost:6379", 0, false},
		{"sub-millisecond TTL", "redis://localhost:6379", time.Nanosecond, false},
		{"negative TTL", "redis://localhost:6379", -time.Second, false},
		{"long TTL", "redis://localhost:6379", 2 * time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(tc.url, tc.ttl)
			if (err == nil) != tc.ok {
				t.Fatal(err)
			}
			if client != nil {
				if client.client.Options().DialerRetries != 1 || client.client.Options().MaxRetries != 0 {
					t.Fatal("Redis retries must be disabled")
				}
				if err = client.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
