package downloader

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseTransmissionURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantURL  string
		wantUser string
		wantPass string
	}{
		{"simple URL", "http://localhost:9091/transmission/rpc", "http://localhost:9091/transmission/rpc", "", ""},
		{"URL with credentials", "user:pass@http://localhost:9091/transmission/rpc", "http://localhost:9091/transmission/rpc", "user", "pass"},
		{"URL without protocol", "localhost:9091", "localhost:9091", "", ""},
		{"URL with @ in wrong position", "http://user@pass@host/path", "http://user@pass@host/path", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotUser, gotPass := ParseTransmissionURL(tt.input)
			if gotURL != tt.wantURL {
				t.Errorf("url = %q, want %q", gotURL, tt.wantURL)
			}
			if gotUser != tt.wantUser {
				t.Errorf("user = %q, want %q", gotUser, tt.wantUser)
			}
			if gotPass != tt.wantPass {
				t.Errorf("password = %q, want %q", gotPass, tt.wantPass)
			}
		})
	}
}

func TestParseAria2URL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantURL   string
		wantToken string
	}{
		{"simple URL", "http://localhost:6800/jsonrpc", "http://localhost:6800/jsonrpc", ""},
		{"URL with token", "mytoken@http://localhost:6800/jsonrpc", "http://localhost:6800/jsonrpc", "mytoken"},
		{"URL without protocol", "localhost:6800", "localhost:6800", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotToken := ParseAria2URL(tt.input)
			if gotURL != tt.wantURL {
				t.Errorf("url = %q, want %q", gotURL, tt.wantURL)
			}
			if gotToken != tt.wantToken {
				t.Errorf("token = %q, want %q", gotToken, tt.wantToken)
			}
		})
	}
}

func TestTransmissionClientAddMagnet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "test-session-id")
			w.WriteHeader(http.StatusConflict)
			return
		}
		resp := map[string]interface{}{"result": "success"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewTransmissionClient(server.Client(), TransmissionConfig{
		URL: server.URL,
	})

	if err := client.AddMagnet("magnet:?xt=urn:btih:test"); err != nil {
		t.Errorf("AddMagnet failed: %v", err)
	}
}

func TestTransmissionClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{"result": "duplicate torrent"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewTransmissionClient(server.Client(), TransmissionConfig{
		URL: server.URL,
	})

	if err := client.AddMagnet("magnet:?xt=urn:btih:test"); err == nil {
		t.Error("expected error for non-success result, got nil")
	}
}

func TestAria2ClientAddMagnet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":      "nyaa-crawler",
			"jsonrpc": "2.0",
			"result":  "208888e6a22de25d",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewAria2Client(server.Client(), Aria2Config{
		URL:   server.URL,
		Token: "testtoken",
	})

	if err := client.AddMagnet("magnet:?xt=urn:btih:test"); err != nil {
		t.Errorf("AddMagnet failed: %v", err)
	}
}

func TestAria2ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":      "nyaa-crawler",
			"jsonrpc": "2.0",
			"error": map[string]interface{}{
				"code":    1,
				"message": "Unauthorized",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewAria2Client(server.Client(), Aria2Config{
		URL:   server.URL,
		Token: "badtoken",
	})

	if err := client.AddMagnet("magnet:?xt=urn:btih:test"); err == nil {
		t.Error("expected error for aria2 error response, got nil")
	}
}
