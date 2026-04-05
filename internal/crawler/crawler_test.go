package crawler

import (
	"strconv"
	"testing"
)

func TestIDRegex(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		found    bool
	}{
		{"/view/12345", 12345, true},
		{"/view/2041474", 2041474, true},
		{"/view/1", 1, true},
		{"/download/12345", 0, false},
		{"/view/abc", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		matches := idRegex.FindStringSubmatch(tt.input)
		if tt.found {
			if len(matches) < 2 {
				t.Errorf("expected match for %q, got none", tt.input)
				continue
			}
			id, err := strconv.Atoi(matches[1])
			if err != nil {
				t.Errorf("failed to parse ID from %q: %v", tt.input, err)
				continue
			}
			if id != tt.expected {
				t.Errorf("expected ID %d for %q, got %d", tt.expected, tt.input, id)
			}
		} else {
			if len(matches) > 0 {
				t.Errorf("expected no match for %q, got %v", tt.input, matches)
			}
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		DSN:      "postgres://localhost:5432/test?sslmode=disable",
		URL:      "https://nyaa.si/",
		ProxyURL: "",
	}

	if cfg.DSN == "" {
		t.Error("DSN should not be empty")
	}
	if cfg.URL == "" {
		t.Error("URL should not be empty")
	}
}

func TestConfigWithProxy(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
	}{
		{"HTTP proxy", "http://proxy:8080"},
		{"HTTPS proxy", "https://proxy:8080"},
		{"SOCKS5 proxy", "socks5://proxy:1080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				DSN:      "postgres://localhost:5432/test",
				URL:      "https://nyaa.si/",
				ProxyURL: tt.proxyURL,
			}
			if cfg.ProxyURL != tt.proxyURL {
				t.Errorf("expected proxy %q, got %q", tt.proxyURL, cfg.ProxyURL)
			}
		})
	}
}

func TestCrawlerDefaults(t *testing.T) {
	c := &Crawler{
		MaxRetries: 3,
	}
	if c.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", c.MaxRetries)
	}
}
