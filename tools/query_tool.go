package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"nyaa-crawler/internal/db"
	"nyaa-crawler/pkg/models"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Define command line flags
	dbPath := flag.String("db", "../nyaa.db", "Path to the SQLite database file")
	searchPattern := flag.String("regex", "", "Text pattern to match in torrent names (using LIKE operator)")
	limit := flag.Int("limit", 10, "Number of results to show")
	transmissionURL := flag.String("transmission", "", "Transmission RPC URL (e.g., user:pass@http://localhost:9091/transmission/rpc)")
	aria2URL := flag.String("aria2", "", "aria2 RPC URL (e.g., token@http://localhost:6800/jsonrpc)")
	dryRun := flag.Bool("dry-run", false, "Show what would be sent to Transmission/aria2 without actually sending")
	flag.Parse()

	// Validate database path
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("Database file does not exist: %s", *dbPath)
	}

	dbs, err := db.NewDBService(*dbPath)
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
	if (*transmissionURL != "" || *aria2URL != "") && !*dryRun {
		processMagnetLinks(dbs, torrents, *transmissionURL, *aria2URL)
	} else if (*transmissionURL != "" || *aria2URL != "") && *dryRun {
		showDryRunInfo(torrents, *transmissionURL, *aria2URL)
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
func processMagnetLinks(dbs *db.DBService, torrents []models.Torrent, transmissionURL, aria2URL string) {
	magnetToIdMap := collectMagnetLinks(torrents, transmissionURL, aria2URL)

	if transmissionURL != "" {
		transmissionLinks := getMagnetLinks(magnetToIdMap)
		sendMagnetLinksToService("Transmission", transmissionLinks, func(link string) error {
			parsedURL, user, pass := parseTransmissionURL(transmissionURL)
			return sendToTransmissionRPC(parsedURL, user, pass, link)
		})
		for _, magnet := range transmissionLinks {
			if id, exists := magnetToIdMap[magnet]; exists {
				dbs.UpdatePushedStatus(id, "pushed_to_transmission")
			}
		}
	}

	if aria2URL != "" {
		aria2Links := getMagnetLinks(magnetToIdMap)
		sendMagnetLinksToService("aria2", aria2Links, func(link string) error {
			parsedURL, token := parseAria2URL(aria2URL)
			return sendToAria2RPC(parsedURL, token, link)
		})
		for _, magnet := range aria2Links {
			if id, exists := magnetToIdMap[magnet]; exists {
				dbs.UpdatePushedStatus(id, "pushed_to_aria2")
			}
		}
	}
}

// collectMagnetLinks collects magnet links for each service
func collectMagnetLinks(torrents []models.Torrent, transmissionURL, aria2URL string) map[string]int {
	magnetToIdMap := make(map[string]int)
	for _, t := range torrents {
		if transmissionURL != "" && !t.PushedToTransmission && t.Magnet != "" {
			magnetToIdMap[t.Magnet] = t.ID
		}
		if aria2URL != "" && !t.PushedToAria2 && t.Magnet != "" {
			magnetToIdMap[t.Magnet] = t.ID
		}
	}
	return magnetToIdMap
}

// getMagnetLinks extracts magnet links from the map
func getMagnetLinks(magnetToIdMap map[string]int) []string {
	links := make([]string, 0, len(magnetToIdMap))
	for magnet := range magnetToIdMap {
		links = append(links, magnet)
	}
	return links
}

// sendMagnetLinksToService sends magnet links to a download service
func sendMagnetLinksToService(serviceName string, links []string, sendFunc func(string) error) {
	if len(links) == 0 {
		return
	}
	fmt.Printf("\nSending %d magnet links to %s...\n", len(links), serviceName)
	successCount := 0
	for i, link := range links {
		if err := sendFunc(link); err != nil {
			fmt.Printf("Failed to send magnet link %d to %s: %v\n", i+1, serviceName, err)
		} else {
			fmt.Printf("Successfully sent magnet link %d to %s\n", i+1, serviceName)
			successCount++
		}
	}
	fmt.Printf("Successfully sent %d out of %d magnet links to %s\n", successCount, len(links), serviceName)
}

// showDryRunInfo shows what would be sent without actually sending
func showDryRunInfo(torrents []models.Torrent, transmissionURL, aria2URL string) {
	magnetToIdMap := collectMagnetLinks(torrents, transmissionURL, aria2URL)
	transmissionLinks := getMagnetLinks(magnetToIdMap)
	aria2Links := getMagnetLinks(magnetToIdMap)

	if transmissionURL != "" && len(transmissionLinks) > 0 {
		fmt.Printf("\nDry run mode - would send %d magnet links to Transmission:\n", len(transmissionLinks))
		for i, link := range transmissionLinks {
			fmt.Printf("%d. %s\n", i+1, link)
		}
	}

	if aria2URL != "" && len(aria2Links) > 0 {
		fmt.Printf("\nDry run mode - would send %d magnet links to aria2:\n", len(aria2Links))
		for i, link := range aria2Links {
			fmt.Printf("%d. %s\n", i+1, link)
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// sendToTransmissionRPC sends a magnet link to Transmission via its RPC API
func sendToTransmissionRPC(url, user, pass, magnet string) error {
	payload := map[string]interface{}{
		"method": "torrent-add",
		"arguments": map[string]interface{}{
			"filename": magnet,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if user != "" && pass != "" {
		req.SetBasicAuth(user, pass)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Transmission: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		sessionID := resp.Header.Get("X-Transmission-Session-Id")
		if sessionID == "" {
			return fmt.Errorf("Transmission returned 409 Conflict but no session ID found")
		}

		req, err = http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Transmission-Session-Id", sessionID)
		if user != "" && pass != "" {
			req.SetBasicAuth(user, pass)
		}

		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request to Transmission (retry): %v", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Transmission returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response JSON: %v", err)
	}

	if result["result"] != "success" {
		return fmt.Errorf("Transmission returned result: %v", result["result"])
	}

	return nil
}

// parseTransmissionURL extracts credentials from URL if present
func parseTransmissionURL(rawURL string) (string, string, string) {
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, "://") {
		atIndex := strings.Index(rawURL, "@")
		protoIndex := strings.Index(rawURL, "://")

		if atIndex < protoIndex {
			parts := strings.SplitN(rawURL, "@", 2)
			if len(parts) != 2 {
				return rawURL, "", ""
			}

			credentials := parts[0]
			urlPart := parts[1]

			creds := strings.SplitN(credentials, ":", 2)
			if len(creds) != 2 {
				return rawURL, "", ""
			}

			username := creds[0]
			password := creds[1]

			return urlPart, username, password
		}
	}

	return rawURL, "", ""
}

// sendToAria2RPC sends a magnet link to aria2 via its JSON-RPC API
func sendToAria2RPC(urlStr, token, magnet string) error {
	params := []interface{}{"token:" + token, []string{magnet}}
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "nyaa-crawler",
		"method":  "aria2.addUri",
		"params":  params,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %v", err)
	}

	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to aria2: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response JSON: %v", err)
	}

	if result["error"] != nil {
		return fmt.Errorf("aria2 returned error: %v", result["error"])
	}

	return nil
}

// parseAria2URL extracts token from URL if present
func parseAria2URL(rawURL string) (string, string) {
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, "://") {
		atIndex := strings.Index(rawURL, "@")
		protoIndex := strings.Index(rawURL, "://")

		if atIndex < protoIndex {
			parts := strings.SplitN(rawURL, "@", 2)
			if len(parts) != 2 {
				return rawURL, ""
			}

			token := parts[0]
			urlPart := parts[1]

			return urlPart, token
		}
	}

	return rawURL, ""
}

// Dummy function to satisfy sql.Rows reference (kept for API compatibility)
func scanTorrents(rows *sql.Rows) []models.Torrent {
	var torrents []models.Torrent
	for rows.Next() {
		var t models.Torrent
		err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Size, &t.Date, &t.Magnet, &t.PushedToTransmission, &t.PushedToAria2)
		if err != nil {
			log.Fatal("Failed to scan row:", err)
		}
		torrents = append(torrents, t)
	}
	return torrents
}
