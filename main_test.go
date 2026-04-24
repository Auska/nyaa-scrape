package main

import (
	"context"
	"os"
	"testing"
	"time"

	"nyaa-crawler/internal/db"
	"nyaa-crawler/pkg/models"
)

// getTestDSN returns the test database connection string
func getTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("NYAA_DB")
	if dsn == "" {
		t.Skip("Skipping test: NYAA_DB not set")
	}
	return dsn
}

// setupTestDB creates a DBService for testing with automatic cleanup
func setupTestDB(t *testing.T) *db.DBService {
	t.Helper()
	dsn := getTestDSN(t)
	dbs, err := db.NewDBService(dsn)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}
	t.Cleanup(func() { dbs.Close() })
	return dbs
}

func TestNewDBService(t *testing.T) {
	dbs := setupTestDB(t)
	_ = dbs
}

func TestInsertAndGetTorrent(t *testing.T) {
	dbs := setupTestDB(t)
	_ = dbs.DeleteAll()

	torrent := models.Torrent{
		ID:       12345,
		Name:     "Test Torrent",
		Magnet:   "magnet:?xt=urn:btih:test",
		Category: "Anime",
		Size:     "1.5GB",
		Date:     "2026-01-13",
	}

	if err := dbs.InsertTorrents([]models.Torrent{torrent}); err != nil {
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
	dbs := setupTestDB(t)
	_ = dbs.DeleteAll()

	torrent := models.Torrent{
		ID:       99999,
		Name:     "Duplicate Test",
		Magnet:   "magnet:?xt=urn:btih:dup",
		Category: "Test",
		Size:     "100MB",
		Date:     "2026-01-13",
	}

	if err := dbs.InsertTorrents([]models.Torrent{torrent}); err != nil {
		t.Fatalf("Failed to insert torrent first time: %v", err)
	}
	if err := dbs.InsertTorrents([]models.Torrent{torrent}); err != nil {
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
	dbs := setupTestDB(t)
	_ = dbs.DeleteAll()

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
	dbs := setupTestDB(t)

	if err := dbs.InsertTorrents([]models.Torrent{}); err != nil {
		t.Errorf("Expected no error on empty batch, got: %v", err)
	}
}

func TestUpdatePushedStatus(t *testing.T) {
	dbs := setupTestDB(t)
	_ = dbs.DeleteAll()

	torrent := models.Torrent{
		ID:       500,
		Name:     "Push Test",
		Magnet:   "magnet:push",
		Category: "Test",
		Size:     "1GB",
		Date:     "2026-01-13",
	}

	if err := dbs.InsertTorrents([]models.Torrent{torrent}); err != nil {
		t.Fatalf("Failed to insert torrent: %v", err)
	}

	// Test valid target
	if err := dbs.UpdatePushedStatus(500, models.PushTargetTransmission); err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test invalid target (should return error)
	if err := dbs.UpdatePushedStatus(500, models.PushTarget("invalid_column")); err == nil {
		t.Error("Expected error for invalid target, got nil")
	}
}

func TestGetTorrentsByPattern(t *testing.T) {
	dbs := setupTestDB(t)
	_ = dbs.DeleteAll()

	torrents := []models.Torrent{
		{ID: 1001, Name: "One Piece Episode 1", Magnet: "magnet:1", Category: "Anime", Size: "1GB", Date: "2026-01-13"},
		{ID: 1002, Name: "Naruto Episode 1", Magnet: "magnet:2", Category: "Anime", Size: "1GB", Date: "2026-01-13"},
		{ID: 1003, Name: "One Piece Episode 2", Magnet: "magnet:3", Category: "Anime", Size: "1GB", Date: "2026-01-13"},
	}

	if err := dbs.InsertTorrents(torrents); err != nil {
		t.Fatalf("Failed to insert torrents: %v", err)
	}

	results, err := dbs.GetTorrentsByPattern("One Piece", 10)
	if err != nil {
		t.Fatalf("Failed to search torrents: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestGetLatestTorrents(t *testing.T) {
	dbs := setupTestDB(t)
	_ = dbs.DeleteAll()

	torrents := []models.Torrent{
		{ID: 2001, Name: "Torrent A", Magnet: "magnet:a", Category: "Anime", Size: "1GB", Date: "2026-01-13"},
		{ID: 2002, Name: "Torrent B", Magnet: "magnet:b", Category: "Anime", Size: "1GB", Date: "2026-01-14"},
		{ID: 2003, Name: "Torrent C", Magnet: "magnet:c", Category: "Anime", Size: "1GB", Date: "2026-01-15"},
	}

	if err := dbs.InsertTorrents(torrents); err != nil {
		t.Fatalf("Failed to insert torrents: %v", err)
	}

	results, err := dbs.GetLatestTorrents(2)
	if err != nil {
		t.Fatalf("Failed to get latest torrents: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0].ID != 2003 {
		t.Errorf("Expected first result ID 2003, got %d", results[0].ID)
	}
}

func TestGetTorrentCount(t *testing.T) {
	dbs := setupTestDB(t)
	_ = dbs.DeleteAll()

	torrents := []models.Torrent{
		{ID: 3001, Name: "With Magnet", Magnet: "magnet:x", Category: "Test", Size: "1GB", Date: "2026-01-13"},
		{ID: 3002, Name: "Without Magnet", Magnet: "", Category: "Test", Size: "1GB", Date: "2026-01-13"},
	}

	if err := dbs.InsertTorrents(torrents); err != nil {
		t.Fatalf("Failed to insert torrents: %v", err)
	}

	total, withMagnet, err := dbs.GetTorrentCount()
	if err != nil {
		t.Fatalf("Failed to get count: %v", err)
	}

	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}
	if withMagnet != 1 {
		t.Errorf("Expected withMagnet 1, got %d", withMagnet)
	}
}

func TestGetMatchCount(t *testing.T) {
	dbs := setupTestDB(t)
	_ = dbs.DeleteAll()

	torrents := []models.Torrent{
		{ID: 4001, Name: "Test Match A", Magnet: "magnet:a", Category: "Test", Size: "1GB", Date: "2026-01-13"},
		{ID: 4002, Name: "No Match", Magnet: "magnet:b", Category: "Test", Size: "1GB", Date: "2026-01-13"},
		{ID: 4003, Name: "Test Match B", Magnet: "magnet:c", Category: "Test", Size: "1GB", Date: "2026-01-13"},
	}

	if err := dbs.InsertTorrents(torrents); err != nil {
		t.Fatalf("Failed to insert torrents: %v", err)
	}

	count, err := dbs.GetMatchCount("Test Match")
	if err != nil {
		t.Fatalf("Failed to get match count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	if ctx.Err() == nil {
		t.Error("Expected context to be cancelled")
	}
}
