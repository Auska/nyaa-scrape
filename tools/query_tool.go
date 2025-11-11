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
	"strings"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Define command line flags
	dbPath := flag.String("db", "../nyaa.db", "Path to the SQLite database file")
	searchPattern := flag.String("regex", "", "Text pattern to match in torrent names (using LIKE operator)")
	limit := flag.Int("limit", 10, "Number of results to show")
	transmissionURL := flag.String("transmission", "", "Transmission RPC URL (e.g., user:pass@http://localhost:9091/transmission/rpc)")
	sendToTransmission := flag.Bool("send", false, "Send magnet links to Transmission")
	dryRun := flag.Bool("dry-run", false, "Show what would be sent to Transmission without actually sending")
	flag.Parse()

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	var rows *sql.Rows
	var query string
	
	if *searchPattern != "" {
		// Use LIKE with % wildcards for pattern matching
		likePattern := "%" + *searchPattern + "%"
		query = "SELECT id, name, category, size, date, magnet FROM torrents WHERE name LIKE ? ORDER BY id DESC LIMIT ?"
		rows, err = db.Query(query, likePattern, *limit)
		if err != nil {
			log.Fatal("Failed to query database:", err)
		}
		fmt.Printf("Torrents matching pattern '%s' (limit %d):\n", *searchPattern, *limit)
	} else {
		query = "SELECT id, name, category, size, date, magnet FROM torrents ORDER BY id DESC LIMIT ?"
		rows, err = db.Query(query, *limit)
		if err != nil {
			log.Fatal("Failed to query database:", err)
		}
		fmt.Printf("Latest %d torrents:\n", *limit)
	}
	
	defer rows.Close()

	fmt.Printf("%-10s %-50s %-25s %-10s %-10s\n", "ID", "Name", "Category", "Size", "Date")
	fmt.Println(strings.Repeat("-", 120))

	var magnetLinks []string
	for rows.Next() {
		var id int
		var name, category, size, date string
		var magnet string

		err := rows.Scan(&id, &name, &category, &size, &date, &magnet)
		if err != nil {
			log.Fatal("Failed to scan row:", err)
		}

		fmt.Printf("%-10d %-50s %-25s %-10s %-10s\n", 
			id, truncateString(name, 49), category, size, date)
		
		// Collect magnet links if we're going to send them to Transmission
		if *sendToTransmission && magnet != "" {
			magnetLinks = append(magnetLinks, magnet)
		}
	}

	// Show matching count if using search pattern
	if *searchPattern != "" {
		var matchCount int
		likePattern := "%" + *searchPattern + "%"
		query = "SELECT COUNT(*) FROM torrents WHERE name LIKE ?"
		err = db.QueryRow(query, likePattern).Scan(&matchCount)
		if err != nil {
			log.Fatal("Failed to get match count:", err)
		}
		fmt.Printf("\nFound %d matching torrents\n", matchCount)
	}

	// Show some statistics
	var total, withMagnet int
	err = db.QueryRow("SELECT COUNT(*), COUNT(CASE WHEN magnet != '' THEN 1 END) FROM torrents").Scan(&total, &withMagnet)
	if err != nil {
		log.Fatal("Failed to get statistics:", err)
	}

	fmt.Printf("Total torrents in database: %d\n", total)
	fmt.Printf("Torrents with magnet links: %d\n", withMagnet)
	
	// Send magnet links to Transmission if requested
	if *sendToTransmission {
		if len(magnetLinks) == 0 {
			fmt.Println("\nNo magnet links found to send to Transmission.")
			return
		}
		
		if *transmissionURL == "" {
			log.Fatal("Transmission URL is required when using -send flag")
		}
		
		if *dryRun {
			fmt.Printf("\nDry run mode - would send %d magnet links to Transmission:\n", len(magnetLinks))
			for i, link := range magnetLinks {
				fmt.Printf("%d. %s\n", i+1, link)
			}
			return
		}
		
		fmt.Printf("\nSending %d magnet links to Transmission...\n", len(magnetLinks))
		// Parse URL for embedded credentials
		parsedURL, user, pass := parseTransmissionURL(*transmissionURL)
		
		successCount := 0
		for i, link := range magnetLinks {
			if err := sendToTransmissionRPC(parsedURL, user, pass, link); err != nil {
				fmt.Printf("Failed to send magnet link %d: %v\n", i+1, err)
			} else {
				fmt.Printf("Successfully sent magnet link %d to Transmission\n", i+1)
				successCount++
			}
		}
		fmt.Printf("Successfully sent %d out of %d magnet links to Transmission\n", successCount, len(magnetLinks))
	}
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

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
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