package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"nyaa-crawler/internal/db"
	"nyaa-crawler/internal/downloader"
	"nyaa-crawler/pkg/models"
)

func main() {
	// Define command line flags
	dsn := flag.String("db", "", "PostgreSQL connection string (or use NYAA_DB env)")
	searchPattern := flag.String("regex", "", "Text pattern to match in torrent names (using LIKE operator)")
	limit := flag.Int("limit", 10, "Number of results to show")
	transmissionURL := flag.String("transmission", "", "Transmission RPC URL (e.g., user:pass@http://localhost:9091/transmission/rpc)")
	aria2URL := flag.String("aria2", "", "aria2 RPC URL (e.g., token@http://localhost:6800/jsonrpc)")
	downloadDir := flag.String("download-dir", "", "Download directory for Transmission and aria2 (e.g., /path/to/downloads)")
	dryRun := flag.Bool("dry-run", false, "Show what would be sent to Transmission/aria2 without actually sending")
	flag.Parse()

	// DSN priority: CLI flag > NYAA_DB env > default
	dsnValue := *dsn
	if dsnValue == "" {
		dsnValue = os.Getenv("NYAA_DB")
	}
	if dsnValue == "" {
		dsnValue = "postgres://localhost:5432/nyaa?sslmode=disable"
	}

	dbs, err := db.NewDBService(dsnValue)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer dbs.Close()

	var torrents []models.Torrent

	if *searchPattern != "" {
		torrents, err = dbs.GetTorrentsByPattern(*searchPattern, *limit)
		if err != nil {
			log.Fatal("Failed to query database:", err)
		}
		fmt.Printf("Torrents matching pattern '%s' (limit %d):\n", *searchPattern, *limit)
	} else {
		torrents, err = dbs.GetLatestTorrents(*limit)
		if err != nil {
			log.Fatal("Failed to query database:", err)
		}
		fmt.Printf("Latest %d torrents:\n", *limit)
	}

	printTorrents(torrents)

	// Show matching count if using search pattern
	if *searchPattern != "" {
		matchCount, err := dbs.GetMatchCount(*searchPattern)
		if err != nil {
			log.Printf("Warning: Failed to get match count: %v", err)
		} else {
			fmt.Printf("\nFound %d matching torrents\n", matchCount)
		}
	}

	// Show some statistics
	total, withMagnet, err := dbs.GetTorrentCount()
	if err != nil {
		log.Printf("Warning: Failed to get statistics: %v", err)
	} else {
		fmt.Printf("Total torrents in database: %d\n", total)
		fmt.Printf("Torrents with magnet links: %d\n", withMagnet)
	}

	// Process magnet links for Transmission and aria2
	if *transmissionURL != "" || *aria2URL != "" {
		if *dryRun {
			showDryRunInfo(torrents, *transmissionURL, *aria2URL, *downloadDir)
		} else {
			processMagnetLinks(dbs, torrents, *transmissionURL, *aria2URL, *downloadDir)
		}
	}
}

// printTorrents prints the torrents in a formatted table
func printTorrents(torrents []models.Torrent) {
	fmt.Printf("%-10s %-50s %-25s %-10s %-10s %-12s %-12s\n", "ID", "Name", "Category", "Size", "Date", "To Trans", "To Aria2")
	fmt.Println(strings.Repeat("-", 135))

	for _, t := range torrents {
		transStatus := "No"
		if t.PushedToTransmission {
			transStatus = "Yes"
		}
		aria2Status := "No"
		if t.PushedToAria2 {
			aria2Status = "Yes"
		}

		fmt.Printf("%-10d %-50s %-25s %-10s %-10s %-12s %-12s\n",
			t.ID, truncateString(t.Name, 49), t.Category, t.Size, t.Date, transStatus, aria2Status)
	}
}

// processMagnetLinks handles sending magnet links to Transmission and/or aria2
func processMagnetLinks(dbs models.DBService, torrents []models.Torrent, transmissionURL, aria2URL, downloadDir string) {
	httpClient := &http.Client{}

	if transmissionURL != "" {
		url, user, pass := downloader.ParseTransmissionURL(transmissionURL)
		client := downloader.NewTransmissionClient(httpClient, downloader.TransmissionConfig{
			URL:         url,
			User:        user,
			Password:    pass,
			DownloadDir: downloadDir,
		})

		sent := 0
		for _, t := range torrents {
			if t.Magnet != "" && !t.PushedToTransmission {
				fmt.Printf("Sending to Transmission: %s\n", truncateString(t.Name, 50))
				if err := client.AddTorrent(t.Magnet); err != nil {
					fmt.Printf("  Failed: %v\n", err)
				} else {
					if err := dbs.UpdatePushedStatus(t.ID, "pushed_to_transmission"); err != nil {
						log.Printf("Failed to update status for id %d: %v", t.ID, err)
					}
					sent++
				}
			}
		}
		fmt.Printf("Sent %d magnet links to Transmission\n", sent)
	}

	if aria2URL != "" {
		url, token := downloader.ParseAria2URL(aria2URL)
		client := downloader.NewAria2Client(httpClient, downloader.Aria2Config{
			URL:         url,
			Token:       token,
			DownloadDir: downloadDir,
		})

		sent := 0
		for _, t := range torrents {
			if t.Magnet != "" && !t.PushedToAria2 {
				fmt.Printf("Sending to aria2: %s\n", truncateString(t.Name, 50))
				if err := client.AddUri(t.Magnet); err != nil {
					fmt.Printf("  Failed: %v\n", err)
				} else {
					if err := dbs.UpdatePushedStatus(t.ID, "pushed_to_aria2"); err != nil {
						log.Printf("Failed to update status for id %d: %v", t.ID, err)
					}
					sent++
				}
			}
		}
		fmt.Printf("Sent %d magnet links to aria2\n", sent)
	}
}

// showDryRunInfo shows what would be sent without actually sending
func showDryRunInfo(torrents []models.Torrent, transmissionURL, aria2URL, downloadDir string) {
	var transmissionCount, aria2Count int

	for _, t := range torrents {
		if t.Magnet != "" {
			if transmissionURL != "" && !t.PushedToTransmission {
				transmissionCount++
			}
			if aria2URL != "" && !t.PushedToAria2 {
				aria2Count++
			}
		}
	}

	if transmissionURL != "" && transmissionCount > 0 {
		fmt.Printf("\nDry run: would send %d magnet links to Transmission", transmissionCount)
		if downloadDir != "" {
			fmt.Printf(" (download directory: %s)", downloadDir)
		}
		fmt.Println()
	}

	if aria2URL != "" && aria2Count > 0 {
		fmt.Printf("\nDry run: would send %d magnet links to aria2", aria2Count)
		if downloadDir != "" {
			fmt.Printf(" (download directory: %s)", downloadDir)
		}
		fmt.Println()
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
