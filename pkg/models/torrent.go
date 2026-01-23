package models

// Torrent represents a torrent entry from Nyaa
type Torrent struct {
	ID                   int
	Name                 string
	Magnet               string
	Category             string
	Size                 string
	Date                 string
	PushedToTransmission bool
	PushedToAria2        bool
}
