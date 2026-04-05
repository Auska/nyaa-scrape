package main

import (
	"context"
	"flag"
	"log"
	"os"

	"nyaa-crawler/internal/crawler"
)

func main() {
	dsn := flag.String("db", "", "PostgreSQL connection string (or use NYAA_DB env)")
	url := flag.String("url", "https://nyaa.si/", "URL to scrape data from")
	proxyURL := flag.String("proxy", "", "Proxy URL (http/https/socks5, or use NYAA_PROXY env)")
	flag.Parse()

	// DSN priority: CLI flag > NYAA_DB env > default
	dsnValue := *dsn
	if dsnValue == "" {
		dsnValue = os.Getenv("NYAA_DB")
	}
	if dsnValue == "" {
		dsnValue = "postgres://localhost:5432/nyaa?sslmode=disable"
	}

	// Proxy priority: CLI flag > NYAA_PROXY env
	proxy := *proxyURL
	if proxy == "" {
		proxy = os.Getenv("NYAA_PROXY")
	}

	cfg := crawler.Config{
		DSN:      dsnValue,
		URL:      *url,
		ProxyURL: proxy,
	}

	log.Printf("Database DSN: %s", cfg.DSN)
	log.Printf("Scraping URL: %s", cfg.URL)

	c, err := crawler.NewCrawler(cfg)
	if err != nil {
		log.Fatal("Failed to create crawler:", err)
	}
	defer c.Close()

	log.Printf("Starting to scrape from web: %s", cfg.URL)

	ctx := context.Background()
	if err := c.ScrapePage(ctx, cfg.URL); err != nil {
		log.Printf("Error scraping: %v", err)
		log.Println("Failed to scrape. Exiting.")
		return
	}

	// Show some results
	torrents, err := c.DBS.GetAllTorrents()
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
