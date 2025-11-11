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
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/proxy"
)

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

	return &DBService{db: db}, nil
}

// InsertTorrent inserts a torrent into the database
func (dbs *DBService) InsertTorrent(torrent Torrent) error {
	stmt, err := dbs.db.Prepare("INSERT OR IGNORE INTO torrents(id, name, magnet, category, size, date) values(?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(torrent.ID, torrent.Name, torrent.Magnet, torrent.Category, torrent.Size, torrent.Date)
	return err
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
	Client *http.Client
	DBS    *DBService
}

// NewCrawler creates a new crawler instance
func NewCrawler(proxyURL string, dbPath string) (*Crawler, error) {
	dbs, err := NewDBService(dbPath)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// If proxy is provided, configure it
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
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
		log.Printf("Using proxy: %s", proxyURL)
	} else {
		log.Println("No proxy configured, using direct connection")
	}

	return &Crawler{
		Client: client,
		DBS:    dbs,
	}, nil
}

// ScrapePage scrapes a single page of torrents
func (c *Crawler) ScrapePage(url string) error {
	resp, err := c.Client.Get(url)
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
	// Define command line flags
	dbPath := flag.String("db", "./nyaa.db", "Path to the SQLite database file")
	url := flag.String("url", "https://nyaa.si/", "URL to scrape data from")
	flag.Parse()

	// Get proxy URL from environment variable
	proxyURL := os.Getenv("PROXY_URL")
	log.Printf("Proxy URL: %s", proxyURL)
	log.Printf("Database path: %s", *dbPath)
	log.Printf("Scraping URL: %s", *url)
	
	crawler, err := NewCrawler(proxyURL, *dbPath)
	if err != nil {
		log.Fatal("Failed to create crawler:", err)
	}
	defer crawler.Close()

	// Scrape from the web instead of local file
	log.Printf("Starting to scrape from web: %s", *url)
	
	if err := crawler.ScrapePage(*url); err != nil {
		log.Printf("Error scraping from web: %v", err)
		log.Println("Failed to scrape from web. Exiting.")
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