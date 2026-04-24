package downloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Downloader defines the interface for adding magnet links to download clients
type Downloader interface {
	AddMagnet(magnet string) error
}

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

// buildPayload constructs the torrent-add JSON payload
func (t *TransmissionClient) buildPayload(magnet string) ([]byte, error) {
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
	return json.Marshal(payload)
}

// buildRequest constructs an HTTP POST request with common headers
func (t *TransmissionClient) buildRequest(payload []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", t.config.URL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.config.User != "" && t.config.Password != "" {
		req.SetBasicAuth(t.config.User, t.config.Password)
	}
	return req, nil
}

// AddMagnet sends a magnet link to Transmission
func (t *TransmissionClient) AddMagnet(magnet string) error {
	payload, err := t.buildPayload(magnet)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := t.buildRequest(payload)
	if err != nil {
		return err
	}

	resp, err := t.client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Transmission: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusConflict {
		return t.handleCSRF(magnet, resp)
	}

	return t.checkResponse(resp)
}

func (t *TransmissionClient) handleCSRF(magnet string, resp *http.Response) error {
	sessionID := resp.Header.Get("X-Transmission-Session-Id")
	if sessionID == "" {
		return fmt.Errorf("transmission returned 409 Conflict but no session ID found in response headers")
	}

	payload, err := t.buildPayload(magnet)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := t.buildRequest(payload)
	if err != nil {
		return err
	}
	req.Header.Set("X-Transmission-Session-Id", sessionID)

	newResp, err := t.client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Transmission (retry): %w", err)
	}
	defer func() { _ = newResp.Body.Close() }()

	return t.checkResponse(newResp)
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
// Supports formats: http://host/path, user:pass@http://host/path
func ParseTransmissionURL(rawURL string) (endpoint, user, password string) {
	// Check if URL contains credentials before the protocol
	atIdx := strings.Index(rawURL, "@")
	protoIdx := strings.Index(rawURL, "://")

	if atIdx < 0 || protoIdx < 0 || atIdx >= protoIdx {
		return rawURL, "", ""
	}

	credPart := rawURL[:atIdx]
	urlPart := rawURL[atIdx+1:]

	u, err := url.Parse(urlPart)
	if err != nil {
		return rawURL, "", ""
	}

	creds := strings.SplitN(credPart, ":", 2)
	if len(creds) != 2 {
		return urlPart, creds[0], ""
	}

	return u.String(), creds[0], creds[1]
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

// AddMagnet sends a magnet link to aria2
func (a *Aria2Client) AddMagnet(magnet string) error {
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
// Supports formats: http://host/path, token@http://host/path
func ParseAria2URL(rawURL string) (endpoint, token string) {
	atIdx := strings.Index(rawURL, "@")
	protoIdx := strings.Index(rawURL, "://")

	if atIdx < 0 || protoIdx < 0 || atIdx >= protoIdx {
		return rawURL, ""
	}

	tokenPart := rawURL[:atIdx]
	urlPart := rawURL[atIdx+1:]

	u, err := url.Parse(urlPart)
	if err != nil {
		return rawURL, ""
	}

	return u.String(), tokenPart
}
