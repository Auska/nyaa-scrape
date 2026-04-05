package crawler

import (
	"strconv"
	"testing"

	"nyaa-crawler/pkg/models"
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
		URL:      "https://nyaa.si/",
		ProxyURL: "",
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
				URL:      "https://nyaa.si/",
				ProxyURL: tt.proxyURL,
			}
			if cfg.ProxyURL != tt.proxyURL {
				t.Errorf("expected proxy %q, got %q", tt.proxyURL, cfg.ProxyURL)
			}
		})
	}
}

func TestCrawlerOptions(t *testing.T) {
	// Test WithMaxRetries option
	c := &Crawler{}
	opt := WithMaxRetries(5)
	if err := opt(c); err != nil {
		t.Errorf("WithMaxRetries returned error: %v", err)
	}
	if c.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", c.MaxRetries)
	}
}

func TestNewCrawlerWithoutDB(t *testing.T) {
	_, err := NewCrawler()
	if err == nil {
		t.Error("expected error when creating crawler without database")
	}
}

// MockDBService is a mock implementation of models.DBService for testing
type MockDBService struct {
	Torrents []models.Torrent
}

func (m *MockDBService) InsertTorrent(torrent models.Torrent) error {
	m.Torrents = append(m.Torrents, torrent)
	return nil
}

func (m *MockDBService) InsertTorrents(torrents []models.Torrent) error {
	m.Torrents = append(m.Torrents, torrents...)
	return nil
}

func (m *MockDBService) GetAllTorrents() ([]models.Torrent, error) {
	return m.Torrents, nil
}

func (m *MockDBService) GetTorrentsByPattern(pattern string, limit int) ([]models.Torrent, error) {
	return m.Torrents, nil
}

func (m *MockDBService) GetLatestTorrents(limit int) ([]models.Torrent, error) {
	return m.Torrents, nil
}

func (m *MockDBService) GetTorrentCount() (total, withMagnet int, err error) {
	return len(m.Torrents), 0, nil
}

func (m *MockDBService) GetMatchCount(pattern string) (int, error) {
	return 0, nil
}

func (m *MockDBService) UpdatePushedStatus(id int, column string) error {
	return nil
}

func (m *MockDBService) Close() {}

func TestNewCrawlerWithMockDB(t *testing.T) {
	mockDB := &MockDBService{}
	c, err := NewCrawler(WithDB(mockDB))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if c.DBS == nil {
		t.Error("expected DB service to be set")
	}
}
