package crawler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"nyaa-crawler/pkg/models"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/proxy"
)

// Pre-compiled regex for ID extraction
var idRegex = regexp.MustCompile(`/view/(\d+)`)

// torrentInserter is the minimal database interface the crawler needs
type torrentInserter interface {
	InsertTorrents(torrents []models.Torrent) error
}

// Crawler handles the scraping logic
type Crawler struct {
	Client     *http.Client
	dbs        torrentInserter
	MaxRetries int
}

// Option is a function that configures the Crawler
type Option func(*Crawler) error

// WithDB sets the database service
func WithDB(dbs torrentInserter) Option {
	return func(c *Crawler) error {
		c.dbs = dbs
		return nil
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(c *Crawler) error {
		c.Client = client
		return nil
	}
}

// WithProxy configures proxy for the crawler
func WithProxy(proxyURL string) Option {
	return func(c *Crawler) error {
		if proxyURL == "" {
			return nil
		}

		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			return fmt.Errorf("error parsing proxy URL: %w", err)
		}

		if c.Client == nil {
			c.Client = &http.Client{Timeout: 30 * time.Second}
		}

		transport, ok := c.Client.Transport.(*http.Transport)
		if !ok {
			transport = &http.Transport{}
		}

		if proxyURLParsed.Scheme == "socks5" {
			dialer, err := proxy.FromURL(proxyURLParsed, proxy.Direct)
			if err != nil {
				return fmt.Errorf("error creating SOCKS5 proxy dialer: %w", err)
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		} else {
			transport.Proxy = http.ProxyURL(proxyURLParsed)
		}
		c.Client.Transport = transport
		log.Printf("Using proxy: %s", proxyURL)
		return nil
	}
}

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(maxRetries int) Option {
	return func(c *Crawler) error {
		c.MaxRetries = maxRetries
		return nil
	}
}

// NewCrawler creates a new crawler instance with options
func NewCrawler(opts ...Option) (*Crawler, error) {
	c := &Crawler{
		Client:     &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 3,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	if c.dbs == nil {
		return nil, fmt.Errorf("database service is required")
	}

	return c, nil
}

// fetchWithRetry performs HTTP GET with automatic retry on failure
// Returns the response body reader if successful
func (c *Crawler) fetchWithRetry(ctx context.Context, targetURL string) (io.ReadCloser, error) {
	var lastErr error

	for attempt := 1; attempt <= c.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.Client.Do(req)
		if err == nil {
			if resp.StatusCode == 200 {
				return resp.Body, nil
			}
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
		} else {
			lastErr = err
		}

		if attempt < c.MaxRetries {
			backoff := time.Duration(attempt) * time.Second
			log.Printf("Attempt %d failed, retrying in %v...", attempt, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	return nil, lastErr
}

// ScrapePage scrapes a single page of torrents
func (c *Crawler) ScrapePage(ctx context.Context, targetURL string) error {
	body, err := c.fetchWithRetry(ctx, targetURL)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", targetURL, err)
	}
	defer func() { _ = body.Close() }()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return err
	}

	return c.processTorrentsFromDoc(doc)
}

// ScrapeFromFile scrapes torrents from a local HTML file
func (c *Crawler) ScrapeFromFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	doc, err := goquery.NewDocumentFromReader(file)
	if err != nil {
		return err
	}

	return c.processTorrentsFromDoc(doc)
}

// processTorrentsFromDoc extracts and inserts torrents from a goquery.Document
func (c *Crawler) processTorrentsFromDoc(doc *goquery.Document) error {
	torrents := ParseTorrents(doc)

	if len(torrents) == 0 {
		log.Println("No torrents found on page")
		return nil
	}

	// Batch insert all torrents
	if err := c.dbs.InsertTorrents(torrents); err != nil {
		return fmt.Errorf("failed to insert torrents: %w", err)
	}

	return nil
}

// ParseTorrents extracts all torrents from a goquery.Document
func ParseTorrents(doc *goquery.Document) []models.Torrent {
	var torrents []models.Torrent

	doc.Find("tbody tr").Each(func(i int, s *goquery.Selection) {
		torrent := ParseTorrentRow(s)
		if torrent != nil {
			torrents = append(torrents, *torrent)
		}
	})

	return torrents
}

// ParseTorrentRow parses a single table row to extract torrent information
func ParseTorrentRow(row *goquery.Selection) *models.Torrent {
	torrent := &models.Torrent{}

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

	// Extract ID from the view link using pre-compiled regex
	href, exists := titleLink.Attr("href")
	if exists {
		matches := idRegex.FindStringSubmatch(href)
		if len(matches) > 1 {
			if _, err := fmt.Sscanf(matches[1], "%d", &torrent.ID); err != nil {
				log.Printf("Warning: failed to parse torrent ID: %v", err)
			}
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

	// Validate that we got a valid ID
	if torrent.ID > 0 {
		return torrent
	}

	return nil
}
