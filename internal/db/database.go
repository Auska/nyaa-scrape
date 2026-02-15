package db

import (
	"database/sql"
	"log"
	"time"

	"nyaa-crawler/pkg/models"

	_ "github.com/lib/pq"
)

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

	// Create torrents table if it doesn't exist
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
	_, err = db.Exec(sqlStmt)
	if err != nil {
		return nil, err
	}

	// Create indexes for better query performance
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_torrents_name ON torrents(name);`,
		`CREATE INDEX IF NOT EXISTS idx_torrents_category ON torrents(category);`,
		`CREATE INDEX IF NOT EXISTS idx_torrents_date ON torrents(date);`,
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			log.Printf("Warning: failed to create index: %v", err)
		}
	}

	return &DBService{db: db}, nil
}

// InsertTorrent inserts a single torrent into the database
func (dbs *DBService) InsertTorrent(torrent models.Torrent) error {
	stmt, err := dbs.db.Prepare("INSERT INTO torrents(id, name, magnet, category, size, date) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(torrent.ID, torrent.Name, torrent.Magnet, torrent.Category, torrent.Size, torrent.Date)
	return err
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
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO torrents(id, name, magnet, category, size, date) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING")
	if err != nil {
		return err
	}
	defer stmt.Close()

	inserted := 0
	for _, t := range torrents {
		if _, err := stmt.Exec(t.ID, t.Name, t.Magnet, t.Category, t.Size, t.Date); err == nil {
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("Batch inserted %d new torrents", inserted)
	return nil
}

// Close closes the database connection
func (dbs *DBService) Close() {
	dbs.db.Close()
}

// GetAllTorrents retrieves all torrents from the database
func (dbs *DBService) GetAllTorrents() ([]models.Torrent, error) {
	rows, err := dbs.db.Query("SELECT id, name, magnet, category, size, date FROM torrents")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
	defer rows.Close()

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
	defer rows.Close()

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
func (dbs *DBService) UpdatePushedStatus(id int, column string) error {
	_, err := dbs.db.Exec("UPDATE torrents SET "+column+" = TRUE WHERE id = $1", id)
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
