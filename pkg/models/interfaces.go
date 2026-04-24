package models

// PushTarget represents the download target for push status updates
type PushTarget string

const (
	// PushTargetTransmission represents the Transmission download client
	PushTargetTransmission PushTarget = "pushed_to_transmission"
	// PushTargetAria2 represents the aria2 download client
	PushTargetAria2 PushTarget = "pushed_to_aria2"
)

// TorrentWriter defines the interface for writing torrent data
type TorrentWriter interface {
	InsertTorrents(torrents []Torrent) error
}

// TorrentReader defines the interface for reading torrent data
type TorrentReader interface {
	GetAllTorrents() ([]Torrent, error)
	GetTorrentsByPattern(pattern string, limit int) ([]Torrent, error)
	GetLatestTorrents(limit int) ([]Torrent, error)
	GetTorrentCount() (total, withMagnet int, err error)
	GetMatchCount(pattern string) (int, error)
}

// TorrentStatusUpdater defines the interface for updating torrent push status
type TorrentStatusUpdater interface {
	UpdatePushedStatus(id int, target PushTarget) error
}

// DBService combines all database interfaces for convenience
type DBService interface {
	TorrentWriter
	TorrentReader
	TorrentStatusUpdater
	Close()
}
