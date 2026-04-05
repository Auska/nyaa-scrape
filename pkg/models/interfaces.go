package models

// TorrentParser defines the interface for parsing torrent data
type TorrentParser interface {
	ParseTorrentRow(row string) *Torrent
}

// DBService defines the interface for database operations
type DBService interface {
	InsertTorrent(torrent Torrent) error
	InsertTorrents(torrents []Torrent) error
	GetAllTorrents() ([]Torrent, error)
	GetTorrentsByPattern(pattern string, limit int) ([]Torrent, error)
	GetLatestTorrents(limit int) ([]Torrent, error)
	GetTorrentCount() (total, withMagnet int, err error)
	GetMatchCount(pattern string) (int, error)
	UpdatePushedStatus(id int, column string) error
	Close()
}
