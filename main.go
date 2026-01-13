package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/proxy"
)

// Config holds application configuration
type Config struct {
	DBPath   string
	URL      string
	ProxyURL string
}

var (
	cfg     Config
	cfgOnce sync.Once
)

// LoadConfig loads configuration from flags and environment variables
func LoadConfig() Config {
	cfgOnce.Do(func() {
		dbPath := flag.String("db", "./nyaa.db", "Path to the SQLite database file")
		url := flag.String("url", "https://nyaa.si/", "URL to scrape data from")
		flag.Parse()

		cfg = Config{
			DBPath:   *dbPath,
			URL:      *url,
			ProxyURL: os.Getenv("PROXY_URL"),
		}
	})

	return cfg
}

// Torrent represents a torrent entry from Nyaa
type Torrent struct {
	ID       int
	Name     string
	Magnet   string
	Category string
	Size     string
	Date     string
}

// DBService handles database operations
type DBService struct {
	db *sql.DB
}

// NewDBService creates a new database service
func NewDBService(dbPath string) (*DBService, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create torrents table if it doesn't exist
	sqlStmt := `CREATE TABLE IF NOT EXISTS torrents (
		id INTEGER PRIMARY KEY,
		name TEXT,
		magnet TEXT,
		category TEXT,
		size TEXT,
		date TEXT,
		pushed_to_transmission BOOLEAN DEFAULT 0,
		pushed_to_aria2 BOOLEAN DEFAULT 0
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
func (dbs *DBService) InsertTorrent(torrent Torrent) error {
	stmt, err := dbs.db.Prepare("INSERT OR IGNORE INTO torrents(id, name, magnet, category, size, date) values(?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(torrent.ID, torrent.Name, torrent.Magnet, torrent.Category, torrent.Size, torrent.Date)
	return err
}

// InsertTorrents inserts multiple torrents in a single transaction
func (dbs *DBService) InsertTorrents(torrents []Torrent) error {
	if len(torrents) == 0 {
		return nil
	}

	tx, err := dbs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT OR IGNORE INTO torrents(id, name, magnet, category, size, date) values(?,?,?,?,?,?)")
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
func (dbs *DBService) GetAllTorrents() ([]Torrent, error) {
	rows, err := dbs.db.Query("SELECT id, name, magnet, category, size, date FROM torrents")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var torrents []Torrent
	for rows.Next() {
		var t Torrent
		err := rows.Scan(&t.ID, &t.Name, &t.Magnet, &t.Category, &t.Size, &t.Date)
		if err != nil {
			return nil, err
		}
		torrents = append(torrents, t)
	}

	return torrents, nil
}

// Crawler handles the scraping logic
type Crawler struct {
	Client    *http.Client
	DBS       *DBService
	MaxRetries int
}

// NewCrawler creates a new crawler instance
func NewCrawler(cfg Config) (*Crawler, error) {
	dbs, err := NewDBService(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// If proxy is provided, configure it
	if cfg.ProxyURL != "" {
		proxyURLParsed, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			log.Printf("Error parsing proxy URL: %v", err)
			return nil, err
		}

		// Check if it's a SOCKS5 proxy
		if proxyURLParsed.Scheme == "socks5" {
			dialer, err := proxy.FromURL(proxyURLParsed, proxy.Direct)
			if err != nil {
				log.Printf("Error creating SOCKS5 proxy dialer: %v", err)
				return nil, err
			}
			transport := &http.Transport{
				Dial: dialer.Dial,
			}
			client.Transport = transport
		} else {
			// For HTTP/HTTPS proxies
			transport := &http.Transport{
				Proxy: http.ProxyURL(proxyURLParsed),
			}
			client.Transport = transport
		}
		log.Printf("Using proxy: %s", cfg.ProxyURL)
	} else {
		log.Println("No proxy configured, using direct connection")
	}

	return &Crawler{
		Client:    client,
		DBS:       dbs,
		MaxRetries: 3,
	}, nil
}

// fetchWithRetry performs HTTP GET with automatic retry on failure
func (c *Crawler) fetchWithRetry(url string) error {
	var lastErr error

	for attempt := 1; attempt <= c.MaxRetries; attempt++ {
		resp, err := c.Client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
			lastErr = fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
		} else {
			lastErr = err
		}

		if attempt < c.MaxRetries {
			backoff := time.Duration(attempt) * time.Second
			log.Printf("Attempt %d failed, retrying in %v...", attempt, backoff)
			time.Sleep(backoff)
		}
	}

	return lastErr
}

// ScrapePage scrapes a single page of torrents
func (c *Crawler) ScrapePage(targetURL string) error {
	// Fetch with retry
	if err := c.fetchWithRetry(targetURL); err != nil {
		return fmt.Errorf("failed to fetch %s: %w", targetURL, err)
	}

	// Make request again to get body
	resp, err := c.Client.Get(targetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	// Find the torrent list table
	doc.Find("tbody tr").Each(func(i int, s *goquery.Selection) {
		torrent := c.parseTorrentRow(s)
		if torrent != nil {
			if err := c.DBS.InsertTorrent(*torrent); err != nil {
				log.Printf("Error inserting torrent %d: %v", torrent.ID, err)
			} else {
				log.Printf("Inserted torrent: %s", torrent.Name)
			}
		}
	})

	return nil
}

// ScrapeFromFile scrapes torrents from a local HTML file
func (c *Crawler) ScrapeFromFile(filePath string) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a document from the file
	doc, err := goquery.NewDocumentFromReader(file)
	if err != nil {
		return err
	}

	// Find the torrent list table
	doc.Find("tbody tr").Each(func(i int, s *goquery.Selection) {
		torrent := c.parseTorrentRow(s)
		if torrent != nil {
			if err := c.DBS.InsertTorrent(*torrent); err != nil {
				log.Printf("Error inserting torrent %d: %v", torrent.ID, err)
			} else {
				log.Printf("Inserted torrent: %s", torrent.Name)
			}
		}
	})

	return nil
}

// parseTorrentRow parses a single table row to extract torrent information
func (c *Crawler) parseTorrentRow(row *goquery.Selection) *Torrent {
	torrent := &Torrent{}

	// Extract name and ID
	// For rows with comments, the title link is the second one
	// For rows without comments, it's the first one
	titleLinks := row.Find("td:nth-child(2) a")
	titleLink := titleLinks.First()

	// Check if first link is a comments link
	firstHref, firstHrefExists := titleLink.Attr("href")
	if firstHrefExists && strings.Contains(firstHref, "#comments") {
		// If first link is comments, use the second link for title
		if titleLinks.Length() > 1 {
			titleLink = titleLinks.Eq(1)
		}
	}

	// Extract ID from the view link
	href, exists := titleLink.Attr("href")
	if exists {
		// Extract ID from href like "/view/2041474"
		re := regexp.MustCompile(`/view/(\d+)`)
		matches := re.FindStringSubmatch(href)
		if len(matches) > 1 {
			fmt.Sscanf(matches[1], "%d", &torrent.ID)
		}
	}

	// Extract name
	torrent.Name = strings.TrimSpace(titleLink.Text())

	// Extract category from the image alt text
	catLink := row.Find("td:first-child a")
	catTitle, exists := catLink.Attr("title")
	if exists {
		torrent.Category = catTitle
	}

	// Extract magnet link
	// Look for all links in the 3rd column (index 2, but nth-child is 1-indexed) and find the one with magnet: prefix
	row.Find("td:nth-child(3) a").Each(func(i int, link *goquery.Selection) {
		href, exists := link.Attr("href")
		if exists && strings.HasPrefix(href, "magnet:") {
			torrent.Magnet = href
		}
	})

	// Extract size
	torrent.Size = strings.TrimSpace(row.Find("td:nth-child(4)").Text())

	// Extract date
	dateCell := row.Find("td:nth-child(5)")
	torrent.Date = strings.TrimSpace(dateCell.Text())

	// Skip seeders, leechers, and downloads as requested

	// Validate that we got a valid ID
	if torrent.ID > 0 {
		return torrent
	}

	return nil
}

// Close closes the crawler resources
func (c *Crawler) Close() {
	c.DBS.Close()
}

func main() {
	cfg := LoadConfig()

	log.Printf("Proxy URL: %s", cfg.ProxyURL)
	log.Printf("Database path: %s", cfg.DBPath)
	log.Printf("Scraping URL: %s", cfg.URL)

	crawler, err := NewCrawler(cfg)
	if err != nil {
		log.Fatal("Failed to create crawler:", err)
	}
	defer crawler.Close()

	log.Printf("Starting to scrape from web: %s", cfg.URL)

	if err := crawler.ScrapePage(cfg.URL); err != nil {
		log.Printf("Error scraping: %v", err)
		log.Println("Failed to scrape. Exiting.")
		return
	}

	// Show some results
	torrents, err := crawler.DBS.GetAllTorrents()
	if err != nil {
		log.Printf("Error retrieving torrents: %v", err)
		return
	}

	log.Printf("Successfully scraped %d torrents", len(torrents))

	// Show first 5 torrents
	for i, t := range torrents {
		if i >= 5 {
			break
		}
		log.Printf("Torrent %d: %s (%s)", t.ID, t.Name, t.Category)
	}
}
