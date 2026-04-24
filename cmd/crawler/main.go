package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"

	"nyaa-crawler/internal/crawler"
	"nyaa-crawler/internal/db"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	dsn := flag.String("db", "", "PostgreSQL connection string (or use NYAA_DB env)")
	scrapeURL := flag.String("url", "https://nyaa.si/", "URL to scrape data from")
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

	log.Printf("Database: %s", sanitizeDSN(dsnValue))
	log.Printf("Scraping URL: %s", *scrapeURL)

	// Create database service
	dbs, err := db.NewDBService(dsnValue)
	if err != nil {
		log.Fatal("Failed to create database service:", err)
	}
	defer dbs.Close()

	// Run database migrations
	if err := dbs.Migrate(); err != nil {
		log.Fatal("Failed to run database migrations:", err)
	}

	// Create crawler with dependency injection
	c, err := crawler.NewCrawler(
		crawler.WithDB(dbs),
		crawler.WithProxy(proxy),
	)
	if err != nil {
		log.Fatal("Failed to create crawler:", err)
	}

	log.Printf("Starting to scrape from web: %s", *scrapeURL)

	ctx := context.Background()
	if err := c.ScrapePage(ctx, *scrapeURL); err != nil {
		log.Printf("Error scraping: %v", err)
		log.Println("Failed to scrape. Exiting.")
		return
	}

	// Show some results
	torrents, err := dbs.GetAllTorrents()
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

// sanitizeDSN masks password in database connection string for safe logging
func sanitizeDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "***"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}
