package downloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTPClient interface for HTTP operations
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client represents a downloader client
type Client struct {
	HTTPClient HTTPClient
}

// NewClient creates a new downloader client
func NewClient(httpClient HTTPClient) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{HTTPClient: httpClient}
}

// TransmissionConfig holds Transmission RPC configuration
type TransmissionConfig struct {
	URL         string
	User        string
	Password    string
	DownloadDir string
}

// TransmissionClient handles Transmission RPC operations
type TransmissionClient struct {
	client *Client
	config TransmissionConfig
}

// NewTransmissionClient creates a new Transmission client
func NewTransmissionClient(httpClient HTTPClient, config TransmissionConfig) *TransmissionClient {
	return &TransmissionClient{
		client: NewClient(httpClient),
		config: config,
	}
}

// AddTorrent sends a magnet link to Transmission
func (t *TransmissionClient) AddTorrent(magnet string) error {
	arguments := map[string]interface{}{
		"filename": magnet,
	}

	if t.config.DownloadDir != "" {
		arguments["download-dir"] = t.config.DownloadDir
	}

	payload := map[string]interface{}{
		"method":    "torrent-add",
		"arguments": arguments,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest("POST", t.config.URL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if t.config.User != "" && t.config.Password != "" {
		req.SetBasicAuth(t.config.User, t.config.Password)
	}

	resp, err := t.client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Transmission: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusConflict {
		return t.handleCSRF(magnet, req, jsonPayload)
	}

	return t.checkResponse(resp)
}

func (t *TransmissionClient) handleCSRF(magnet string, originalReq *http.Request, jsonPayload []byte) error {
	sessionID := originalReq.Header.Get("X-Transmission-Session-Id")
	if sessionID == "" {
		return fmt.Errorf("transmission returned 409 Conflict but no session ID found")
	}

	arguments := map[string]interface{}{"filename": magnet}
	if t.config.DownloadDir != "" {
		arguments["download-dir"] = t.config.DownloadDir
	}
	payload := map[string]interface{}{
		"method":    "torrent-add",
		"arguments": arguments,
	}
	newPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", t.config.URL, bytes.NewBuffer(newPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Transmission-Session-Id", sessionID)
	if t.config.User != "" && t.config.Password != "" {
		req.SetBasicAuth(t.config.User, t.config.Password)
	}

	resp, err := t.client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Transmission (retry): %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return t.checkResponse(resp)
}

func (t *TransmissionClient) checkResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("transmission returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response JSON: %w", err)
	}

	if result["result"] != "success" {
		return fmt.Errorf("transmission returned result: %v", result["result"])
	}

	return nil
}

// ParseTransmissionURL extracts credentials from URL if present
func ParseTransmissionURL(rawURL string) (url, user, password string) {
	if !strings.Contains(rawURL, "@") || !strings.Contains(rawURL, "://") {
		return rawURL, "", ""
	}

	atIndex := strings.Index(rawURL, "@")
	protoIndex := strings.Index(rawURL, "://")

	if atIndex >= protoIndex {
		return rawURL, "", ""
	}

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

	return urlPart, creds[0], creds[1]
}

// Aria2Config holds aria2 RPC configuration
type Aria2Config struct {
	URL         string
	Token       string
	DownloadDir string
}

// Aria2Client handles aria2 RPC operations
type Aria2Client struct {
	client *Client
	config Aria2Config
}

// NewAria2Client creates a new aria2 client
func NewAria2Client(httpClient HTTPClient, config Aria2Config) *Aria2Client {
	return &Aria2Client{
		client: NewClient(httpClient),
		config: config,
	}
}

// AddUri sends a magnet link to aria2
func (a *Aria2Client) AddUri(magnet string) error {
	options := make([]map[string]string, 0)
	if a.config.DownloadDir != "" {
		options = append(options, map[string]string{"dir": a.config.DownloadDir})
	}

	params := []interface{}{"token:" + a.config.Token, []string{magnet}, options}
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "nyaa-crawler",
		"method":  "aria2.addUri",
		"params":  params,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest("POST", a.config.URL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to aria2: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response JSON: %w", err)
	}

	if result["error"] != nil {
		return fmt.Errorf("aria2 returned error: %v", result["error"])
	}

	return nil
}

// ParseAria2URL extracts token from URL if present
func ParseAria2URL(rawURL string) (url, token string) {
	if !strings.Contains(rawURL, "@") || !strings.Contains(rawURL, "://") {
		return rawURL, ""
	}

	atIndex := strings.Index(rawURL, "@")
	protoIndex := strings.Index(rawURL, "://")

	if atIndex >= protoIndex {
		return rawURL, ""
	}

	parts := strings.SplitN(rawURL, "@", 2)
	if len(parts) != 2 {
		return rawURL, ""
	}

	return parts[1], parts[0]
}
