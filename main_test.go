package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"nyaa-crawler/internal/crawler"
	"nyaa-crawler/internal/db"
	"nyaa-crawler/pkg/models"
)

func TestNewDBService(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	dbs, err := db.NewDBService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}
	defer dbs.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestInsertAndGetTorrent(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	dbs, err := db.NewDBService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}
	defer dbs.Close()

	torrent := models.Torrent{
		ID:       12345,
		Name:     "Test Torrent",
		Magnet:   "magnet:?xt=urn:btih:test",
		Category: "Anime",
		Size:     "1.5GB",
		Date:     "2026-01-13",
	}

	if err := dbs.InsertTorrent(torrent); err != nil {
		t.Fatalf("Failed to insert torrent: %v", err)
	}

	torrents, err := dbs.GetAllTorrents()
	if err != nil {
		t.Fatalf("Failed to get torrents: %v", err)
	}

	if len(torrents) != 1 {
		t.Errorf("Expected 1 torrent, got %d", len(torrents))
	}

	if torrents[0].ID != torrent.ID {
		t.Errorf("Expected torrent ID %d, got %d", torrent.ID, torrents[0].ID)
	}
}

func TestInsertDuplicateTorrent(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	dbs, err := db.NewDBService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}
	defer dbs.Close()

	torrent := models.Torrent{
		ID:       99999,
		Name:     "Duplicate Test",
		Magnet:   "magnet:?xt=urn:btih:dup",
		Category: "Test",
		Size:     "100MB",
		Date:     "2026-01-13",
	}

	if err := dbs.InsertTorrent(torrent); err != nil {
		t.Fatalf("Failed to insert torrent first time: %v", err)
	}
	if err := dbs.InsertTorrent(torrent); err != nil {
		t.Fatalf("Failed to insert torrent second time: %v", err)
	}

	torrents, err := dbs.GetAllTorrents()
	if err != nil {
		t.Fatalf("Failed to get torrents: %v", err)
	}

	if len(torrents) != 1 {
		t.Errorf("Expected 1 torrent (no duplicates), got %d", len(torrents))
	}
}

func TestInsertTorrentsBatch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	dbs, err := db.NewDBService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}
	defer dbs.Close()

	torrents := []models.Torrent{
		{ID: 100, Name: "Batch 1", Magnet: "magnet:1", Category: "A", Size: "1GB", Date: "2026-01-13"},
		{ID: 200, Name: "Batch 2", Magnet: "magnet:2", Category: "B", Size: "2GB", Date: "2026-01-13"},
		{ID: 300, Name: "Batch 3", Magnet: "magnet:3", Category: "C", Size: "3GB", Date: "2026-01-13"},
	}

	if err := dbs.InsertTorrents(torrents); err != nil {
		t.Fatalf("Failed to batch insert: %v", err)
	}

	result, err := dbs.GetAllTorrents()
	if err != nil {
		t.Fatalf("Failed to get torrents: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 torrents, got %d", len(result))
	}
}

func TestInsertEmptyBatch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	dbs, err := db.NewDBService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}
	defer dbs.Close()

	if err := dbs.InsertTorrents([]models.Torrent{}); err != nil {
		t.Errorf("Expected no error on empty batch, got: %v", err)
	}
}

func TestLoadConfig(t *testing.T) {
	originalArgs := os.Args
	originalProxy := os.Getenv("PROXY_URL")
	defer func() {
		os.Args = originalArgs
		if originalProxy != "" {
			os.Setenv("PROXY_URL", originalProxy)
		} else {
			os.Unsetenv("PROXY_URL")
		}
	}()

	os.Setenv("PROXY_URL", "socks5://test:1080")
	os.Args = []string{"test", "-db", "/test.db", "-url", "https://test.com"}

	cfg := crawler.LoadConfig()

	if cfg.DBPath != "/test.db" {
		t.Errorf("Expected DBPath /test.db, got %s", cfg.DBPath)
	}
	if cfg.URL != "https://test.com" {
		t.Errorf("Expected URL https://test.com, got %s", cfg.URL)
	}
	if cfg.ProxyURL != "socks5://test:1080" {
		t.Errorf("Expected ProxyURL socks5://test:1080, got %s", cfg.ProxyURL)
	}
}

func TestDatabaseIndexesCreated(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	_, err := db.NewDBService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	indexes := []string{"idx_torrents_name", "idx_torrents_category", "idx_torrents_date"}
	for _, idx := range indexes {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to check index %s: %v", idx, err)
		}
		if count != 1 {
			t.Errorf("Expected index %s to exist, count=%d", idx, count)
		}
	}
}
