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

// mockTorrentInserter is a mock implementation of torrentInserter for testing
type mockTorrentInserter struct {
	Torrents []models.Torrent
}

func (m *mockTorrentInserter) InsertTorrents(torrents []models.Torrent) error {
	m.Torrents = append(m.Torrents, torrents...)
	return nil
}

func TestNewCrawlerWithMockDB(t *testing.T) {
	mockDB := &mockTorrentInserter{}
	c, err := NewCrawler(WithDB(mockDB))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if c.dbs == nil {
		t.Error("expected DB service to be set")
	}
}
