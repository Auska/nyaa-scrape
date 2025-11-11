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
	_ "github.com/mattn/go-sqlite3"
)

// Torrent represents a torrent record from the database
type Torrent struct {
	ID                  int
	Name                string
	Category            string
	Size                string
	Date                string
	Magnet              string
	PushedToTransmission bool
	PushedToAria2       bool
}

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

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	var torrents []Torrent
	var query string

	if *searchPattern != "" {
		// Use LIKE with % wildcards for pattern matching
		likePattern := "%" + *searchPattern + "%"
		query = "SELECT id, name, category, size, date, magnet, pushed_to_transmission, pushed_to_aria2 FROM torrents WHERE name LIKE ? ORDER BY id DESC LIMIT ?"
		rows, err := db.Query(query, likePattern, *limit)
		if err != nil {
			log.Fatal("Failed to query database:", err)
		}
		defer rows.Close()

		torrents = scanTorrents(rows)
		fmt.Printf("Torrents matching pattern '%s' (limit %d):\n", *searchPattern, *limit)
	} else {
		query = "SELECT id, name, category, size, date, magnet, pushed_to_transmission, pushed_to_aria2 FROM torrents ORDER BY id DESC LIMIT ?"
		rows, err := db.Query(query, *limit)
		if err != nil {
			log.Fatal("Failed to query database:", err)
		}
		defer rows.Close()

		torrents = scanTorrents(rows)
		fmt.Printf("Latest %d torrents:\n", *limit)
	}

	printTorrents(torrents)

	// Show matching count if using search pattern
	if *searchPattern != "" {
		var matchCount int
		likePattern := "%" + *searchPattern + "%"
		query = "SELECT COUNT(*) FROM torrents WHERE name LIKE ?"
		err = db.QueryRow(query, likePattern).Scan(&matchCount)
		if err != nil {
			log.Printf("Warning: Failed to get match count: %v", err)
		} else {
			fmt.Printf("\nFound %d matching torrents\n", matchCount)
		}
	}

	// Show some statistics
	var total, withMagnet int
	err = db.QueryRow("SELECT COUNT(*), COUNT(CASE WHEN magnet != '' THEN 1 END) FROM torrents").Scan(&total, &withMagnet)
	if err != nil {
		log.Printf("Warning: Failed to get statistics: %v", err)
	} else {
		fmt.Printf("Total torrents in database: %d\n", total)
		fmt.Printf("Torrents with magnet links: %d\n", withMagnet)
	}

	// Process magnet links for Transmission and aria2
	if (*transmissionURL != "" || *aria2URL != "") && !*dryRun {
		processMagnetLinks(db, torrents, *transmissionURL, *aria2URL)
	} else if (*transmissionURL != "" || *aria2URL != "") && *dryRun {
		showDryRunInfo(torrents, *transmissionURL, *aria2URL)
	}
}

// scanTorrents reads all torrent records from the rows
func scanTorrents(rows *sql.Rows) []Torrent {
	var torrents []Torrent
	for rows.Next() {
		var t Torrent
		err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Size, &t.Date, &t.Magnet, &t.PushedToTransmission, &t.PushedToAria2)
		if err != nil {
			log.Fatal("Failed to scan row:", err)
		}
		torrents = append(torrents, t)
	}
	return torrents
}

// printTorrents prints the torrents in a formatted table
func printTorrents(torrents []Torrent) {
	fmt.Printf("%-10s %-50s %-25s %-10s %-10s %-12s %-12s\n", "ID", "Name", "Category", "Size", "Date", "To Trans", "To Aria2")
	fmt.Println(strings.Repeat("-", 135))

	for _, t := range torrents {
		// Format the push status
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
func processMagnetLinks(db *sql.DB, torrents []Torrent, transmissionURL, aria2URL string) {
	// Create a map to track which magnet links correspond to which IDs
	magnetToIdMap := make(map[string]int)
	var transmissionMagnetLinks, aria2MagnetLinks []string
	
	// Collect magnet links for each service
	for _, t := range torrents {
		if transmissionURL != "" && !t.PushedToTransmission && t.Magnet != "" {
			transmissionMagnetLinks = append(transmissionMagnetLinks, t.Magnet)
			magnetToIdMap[t.Magnet] = t.ID
		}
		if aria2URL != "" && !t.PushedToAria2 && t.Magnet != "" {
			aria2MagnetLinks = append(aria2MagnetLinks, t.Magnet)
			magnetToIdMap[t.Magnet] = t.ID // Update map even if already exists
		}
	}

	// Send magnet links to Transmission
	if transmissionURL != "" && len(transmissionMagnetLinks) > 0 {
		fmt.Printf("\nSending %d magnet links to Transmission...\n", len(transmissionMagnetLinks))
		parsedURL, user, pass := parseTransmissionURL(transmissionURL)
		
		successCount := 0
		for i, link := range transmissionMagnetLinks {
			if err := sendToTransmissionRPC(parsedURL, user, pass, link); err != nil {
				fmt.Printf("Failed to send magnet link %d to Transmission: %v\n", i+1, err)
			} else {
				fmt.Printf("Successfully sent magnet link %d to Transmission\n", i+1)
				successCount++
			}
		}
		fmt.Printf("Successfully sent %d out of %d magnet links to Transmission\n", successCount, len(transmissionMagnetLinks))
		
		// Update database to mark torrents as pushed to Transmission
		if successCount > 0 {
			// Use a transaction to ensure atomic updates
			tx, err := db.Begin()
			if err != nil {
				log.Printf("Failed to begin transaction: %v", err)
			} else {
				updateStmt, err := tx.Prepare("UPDATE torrents SET pushed_to_transmission = 1 WHERE id = ?")
				if err != nil {
					log.Printf("Failed to prepare update statement: %v", err)
					tx.Rollback()
				} else {
					defer updateStmt.Close()
					
					// Update only the torrents that were successfully sent
					updatedCount := 0
					for i := 0; i < successCount; i++ {
						magnet := transmissionMagnetLinks[i]
						id, exists := magnetToIdMap[magnet]
						if exists {
							if _, err := updateStmt.Exec(id); err != nil {
								log.Printf("Failed to update torrent ID %d: %v", id, err)
							} else {
								fmt.Printf("Marked torrent ID %d as pushed to Transmission in database\n", id)
								updatedCount++
							}
						}
					}
					// Commit the transaction
					if err := tx.Commit(); err != nil {
						log.Printf("Failed to commit transaction: %v", err)
					} else {
						fmt.Printf("Successfully updated %d torrent records in database\n", updatedCount)
					}
				}
			}
		}
	}
	
	// Send magnet links to aria2
	if aria2URL != "" && len(aria2MagnetLinks) > 0 {
		fmt.Printf("\nSending %d magnet links to aria2...\n", len(aria2MagnetLinks))
		parsedURL, token := parseAria2URL(aria2URL)
		
		successCount := 0
		for i, link := range aria2MagnetLinks {
			if err := sendToAria2RPC(parsedURL, token, link); err != nil {
				fmt.Printf("Failed to send magnet link %d to aria2: %v\n", i+1, err)
			} else {
				fmt.Printf("Successfully sent magnet link %d to aria2\n", i+1)
				successCount++
			}
		}
		fmt.Printf("Successfully sent %d out of %d magnet links to aria2\n", successCount, len(aria2MagnetLinks))
		
		// Update database to mark torrents as pushed to aria2
		if successCount > 0 {
			// Use a transaction to ensure atomic updates
			tx, err := db.Begin()
			if err != nil {
				log.Printf("Failed to begin transaction: %v", err)
			} else {
				updateStmt, err := tx.Prepare("UPDATE torrents SET pushed_to_aria2 = 1 WHERE id = ?")
				if err != nil {
					log.Printf("Failed to prepare update statement: %v", err)
					tx.Rollback()
				} else {
					defer updateStmt.Close()
					
					// Update only the torrents that were successfully sent
					updatedCount := 0
					for i := 0; i < successCount; i++ {
						magnet := aria2MagnetLinks[i]
						id, exists := magnetToIdMap[magnet]
						if exists {
							if _, err := updateStmt.Exec(id); err != nil {
								log.Printf("Failed to update torrent ID %d: %v", id, err)
							} else {
								fmt.Printf("Marked torrent ID %d as pushed to aria2 in database\n", id)
								updatedCount++
							}
						}
					}
					// Commit the transaction
					if err := tx.Commit(); err != nil {
						log.Printf("Failed to commit transaction: %v", err)
					} else {
						fmt.Printf("Successfully updated %d torrent records in database\n", updatedCount)
					}
				}
			}
		}
	}
}

// showDryRunInfo shows what would be sent without actually sending
func showDryRunInfo(torrents []Torrent, transmissionURL, aria2URL string) {
	// Create a map to track which magnet links correspond to which IDs
	magnetToIdMap := make(map[string]int)
	var transmissionMagnetLinks, aria2MagnetLinks []string
	
	// Collect magnet links for each service
	for _, t := range torrents {
		if transmissionURL != "" && !t.PushedToTransmission && t.Magnet != "" {
			transmissionMagnetLinks = append(transmissionMagnetLinks, t.Magnet)
			magnetToIdMap[t.Magnet] = t.ID
		}
		if aria2URL != "" && !t.PushedToAria2 && t.Magnet != "" {
			aria2MagnetLinks = append(aria2MagnetLinks, t.Magnet)
			magnetToIdMap[t.Magnet] = t.ID // Update map even if already exists
		}
	}

	if transmissionURL != "" && len(transmissionMagnetLinks) > 0 {
		fmt.Printf("\nDry run mode - would send %d magnet links to Transmission:\n", len(transmissionMagnetLinks))
		for i, link := range transmissionMagnetLinks {
			fmt.Printf("%d. %s\n", i+1, link)
		}
	}

	if aria2URL != "" && len(aria2MagnetLinks) > 0 {
		fmt.Printf("\nDry run mode - would send %d magnet links to aria2:\n", len(aria2MagnetLinks))
		for i, link := range aria2MagnetLinks {
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
	// Prepare the RPC request payload
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
	
	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if user != "" && pass != "" {
		req.SetBasicAuth(user, pass)
	}
	
	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Transmission: %v", err)
	}
	defer resp.Body.Close()
	
	// Check if we got a 409 Conflict error (session ID issue)
	if resp.StatusCode == http.StatusConflict {
		// Extract the session ID from the response header
		sessionID := resp.Header.Get("X-Transmission-Session-Id")
		if sessionID == "" {
			return fmt.Errorf("Transmission returned 409 Conflict but no session ID found")
		}
		
		// Retry the request with the session ID
		// We need to recreate the request body since it was consumed
		req, err = http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %v", err)
		}
		
		// Set headers again
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Transmission-Session-Id", sessionID)
		if user != "" && pass != "" {
			req.SetBasicAuth(user, pass)
		}
		
		// Send the retry request
		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request to Transmission (retry): %v", err)
		}
		defer resp.Body.Close()
	}
	
	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Transmission returned status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
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
	// Check if URL contains credentials in format "user:pass@http://host"
	// This handles the format "user:pass@http://host:port/path"
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, "://") {
		// Find the position of the first "@"
		atIndex := strings.Index(rawURL, "@")
		// Find the position of the first "://"
		protoIndex := strings.Index(rawURL, "://")
		
		// If "@" comes before "://", it means credentials are at the beginning
		if atIndex < protoIndex {
			// Split by "@" to separate credentials from the URL
			parts := strings.SplitN(rawURL, "@", 2)
			if len(parts) != 2 {
				return rawURL, "", ""
			}
			
			credentials := parts[0]
			urlPart := parts[1]
			
			// Split credentials by ":" to get user and pass
			creds := strings.SplitN(credentials, ":", 2)
			if len(creds) != 2 {
				return rawURL, "", ""
			}
			
			username := creds[0]
			password := creds[1]
			
			// Return the URL part as the URL, and extracted credentials
			return urlPart, username, password
		}
	}
	
	// No credentials found in the expected format, return as-is
	return rawURL, "", ""
}

// sendToAria2RPC sends a magnet link to aria2 via its JSON-RPC API
func sendToAria2RPC(urlStr, token, magnet string) error {
	// Prepare the RPC request payload with token as first parameter
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
	
	// Create HTTP request
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	
	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to aria2: %v", err)
	}
	defer resp.Body.Close()
	
	// Check response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	
	// Check if response is valid JSON and contains no error
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response JSON: %v", err)
	}
	
	// Check if there's an error in the response
	if result["error"] != nil {
		return fmt.Errorf("aria2 returned error: %v", result["error"])
	}
	
	// Success
	return nil
}

// parseAria2URL extracts token from URL if present
func parseAria2URL(rawURL string) (string, string) {
	// Check if URL contains token in format "token@http://host"
	// This handles the format "token@http://host:port/path"
	if strings.Contains(rawURL, "@") && strings.Contains(rawURL, "://") {
		// Find the position of the first "@"
		atIndex := strings.Index(rawURL, "@")
		// Find the position of the first "://"
		protoIndex := strings.Index(rawURL, "://")
		
		// If "@" comes before "://", it means token is at the beginning
		if atIndex < protoIndex {
			// Split by "@" to separate token from the URL
			parts := strings.SplitN(rawURL, "@", 2)
			if len(parts) != 2 {
				return rawURL, ""
			}
			
			token := parts[0]
			urlPart := parts[1]
			
			// Return the URL part as the URL, and extracted token
			return urlPart, token
		}
	}
	
	// No token found in the expected format, return as-is
	return rawURL, ""
}