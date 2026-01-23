package main

import (
	"flag"
	"log"

	"nyaa-crawler/internal/crawler"
)

func main() {
	dbPath := flag.String("db", "./nyaa.db", "Path to the SQLite database file")
	url := flag.String("url", "https://nyaa.si/", "URL to scrape data from")
	flag.Parse()

	cfg := crawler.Config{
		DBPath:   *dbPath,
		URL:      *url,
		ProxyURL: "",
	}

	log.Printf("Database path: %s", cfg.DBPath)
	log.Printf("Scraping URL: %s", cfg.URL)

	c, err := crawler.NewCrawler(cfg)
	if err != nil {
		log.Fatal("Failed to create crawler:", err)
	}
	defer c.Close()

	log.Printf("Starting to scrape from web: %s", cfg.URL)

	if err := c.ScrapePage(cfg.URL); err != nil {
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
