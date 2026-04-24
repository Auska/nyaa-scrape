package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"nyaa-crawler/pkg/models"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

// Verify DBService implements models.DBService interface
var _ models.DBService = (*DBService)(nil)

// DBService handles database operations
type DBService struct {
	db *sql.DB
}

// NewDBService creates a new database service
func NewDBService(connStr string) (*DBService, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DBService{db: db}, nil
}

// Migrate creates tables and indexes if they don't exist
func (dbs *DBService) Migrate() error {
	sqlStmt := `CREATE TABLE IF NOT EXISTS torrents (
		id INTEGER PRIMARY KEY,
		name TEXT,
		magnet TEXT,
		category TEXT,
		size TEXT,
		date TEXT,
		pushed_to_transmission BOOLEAN DEFAULT FALSE,
		pushed_to_aria2 BOOLEAN DEFAULT FALSE
	);`
	if _, err := dbs.db.Exec(sqlStmt); err != nil {
		return fmt.Errorf("failed to create torrents table: %w", err)
	}

	// Create indexes for better query performance
	// Note: B-tree index on name is ineffective for LIKE '%pattern%' queries.
	// For full-text search, consider using pg_trgm GIN index or tsvector.
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_torrents_category ON torrents(category);`,
		`CREATE INDEX IF NOT EXISTS idx_torrents_date ON torrents(date);`,
	}
	for _, idx := range indexes {
		if _, err := dbs.db.Exec(idx); err != nil {
			log.Printf("Warning: failed to create index: %v", err)
		}
	}

	return nil
}

// InsertTorrents inserts multiple torrents in a single transaction
func (dbs *DBService) InsertTorrents(torrents []models.Torrent) error {
	if len(torrents) == 0 {
		return nil
	}

	tx, err := dbs.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare("INSERT INTO torrents(id, name, magnet, category, size, date) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING")
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	var insertErrs []error
	inserted := 0
	for _, t := range torrents {
		result, err := stmt.Exec(t.ID, t.Name, t.Magnet, t.Category, t.Size, t.Date)
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				// unique_violation: expected with ON CONFLICT DO NOTHING, skip
				continue
			}
			insertErrs = append(insertErrs, fmt.Errorf("torrent %d: %w", t.ID, err))
			continue
		}
		affected, _ := result.RowsAffected()
		inserted += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if len(insertErrs) > 0 {
		log.Printf("Batch insert completed with %d errors out of %d torrents", len(insertErrs), len(torrents))
		for _, e := range insertErrs {
			log.Printf("  Insert error: %v", e)
		}
	}
	log.Printf("Batch inserted %d new torrents", inserted)
	return nil
}

// Close closes the database connection
func (dbs *DBService) Close() {
	_ = dbs.db.Close()
}

// GetAllTorrents retrieves torrents from the database with a safety limit
func (dbs *DBService) GetAllTorrents() ([]models.Torrent, error) {
	rows, err := dbs.db.Query("SELECT id, name, magnet, category, size, date FROM torrents LIMIT 10000")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var torrents []models.Torrent
	for rows.Next() {
		var t models.Torrent
		err := rows.Scan(&t.ID, &t.Name, &t.Magnet, &t.Category, &t.Size, &t.Date)
		if err != nil {
			return nil, err
		}
		torrents = append(torrents, t)
	}

	return torrents, nil
}

// GetTorrentsByPattern retrieves torrents matching a pattern
func (dbs *DBService) GetTorrentsByPattern(pattern string, limit int) ([]models.Torrent, error) {
	likePattern := "%" + pattern + "%"
	rows, err := dbs.db.Query(
		"SELECT id, name, category, size, date, magnet, pushed_to_transmission, pushed_to_aria2 FROM torrents WHERE name LIKE $1 ORDER BY id DESC LIMIT $2",
		likePattern, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanTorrents(rows)
}

// GetLatestTorrents retrieves the latest torrents
func (dbs *DBService) GetLatestTorrents(limit int) ([]models.Torrent, error) {
	rows, err := dbs.db.Query(
		"SELECT id, name, category, size, date, magnet, pushed_to_transmission, pushed_to_aria2 FROM torrents ORDER BY id DESC LIMIT $1",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanTorrents(rows)
}

// GetTorrentCount returns the total count and magnet count
func (dbs *DBService) GetTorrentCount() (total, withMagnet int, err error) {
	err = dbs.db.QueryRow("SELECT COUNT(*), COUNT(CASE WHEN magnet != '' THEN 1 END) FROM torrents").Scan(&total, &withMagnet)
	return
}

// GetMatchCount returns the count of torrents matching a pattern
func (dbs *DBService) GetMatchCount(pattern string) (int, error) {
	likePattern := "%" + pattern + "%"
	var count int
	err := dbs.db.QueryRow("SELECT COUNT(*) FROM torrents WHERE name LIKE $1", likePattern).Scan(&count)
	return count, err
}

// UpdatePushedStatus updates the pushed status for a torrent
func (dbs *DBService) UpdatePushedStatus(id int, target models.PushTarget) error {
	column := string(target)
	if target != models.PushTargetTransmission && target != models.PushTargetAria2 {
		return fmt.Errorf("invalid push target: %s", column)
	}
	_, err := dbs.db.Exec("UPDATE torrents SET "+column+" = TRUE WHERE id = $1", id)
	return err
}

// DeleteAll removes all torrents from the database (for testing only)
func (dbs *DBService) DeleteAll() error {
	_, err := dbs.db.Exec("DELETE FROM torrents")
	return err
}

// scanTorrents reads all torrent records from the rows
func scanTorrents(rows *sql.Rows) ([]models.Torrent, error) {
	var torrents []models.Torrent
	for rows.Next() {
		var t models.Torrent
		err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Size, &t.Date, &t.Magnet, &t.PushedToTransmission, &t.PushedToAria2)
		if err != nil {
			return nil, err
		}
		torrents = append(torrents, t)
	}
	return torrents, nil
}
