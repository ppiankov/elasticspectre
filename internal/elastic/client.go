package elastic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Flavor distinguishes Elasticsearch from OpenSearch.
type Flavor string

const (
	Elasticsearch Flavor = "elasticsearch"
	OpenSearch    Flavor = "opensearch"
)

// ClusterInfo holds the result of GET /.
type ClusterInfo struct {
	Name    string
	Flavor  Flavor
	Version string
}

// Options configures the client connection.
type Options struct {
	URL      string
	CloudID  string
	Username string
	Password string
	APIKey   string
}

// Client is a thin HTTP wrapper for ES/OpenSearch REST APIs.
type Client struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
	apiKey     string
}

// New creates a Client from Options. Resolves Cloud ID to a URL if provided.
func New(opts Options) (*Client, error) {
	if opts.URL == "" && opts.CloudID == "" {
		return nil, fmt.Errorf("either URL or CloudID is required")
	}
	if opts.URL != "" && opts.CloudID != "" {
		return nil, fmt.Errorf("URL and CloudID are mutually exclusive")
	}

	baseURL := opts.URL
	if opts.CloudID != "" {
		resolved, err := resolveCloudID(opts.CloudID)
		if err != nil {
			return nil, fmt.Errorf("resolving cloud ID: %w", err)
		}
		baseURL = resolved
	}

	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		username:   opts.Username,
		password:   opts.Password,
		apiKey:     opts.APIKey,
	}, nil
}

// SetHTTPClient replaces the default HTTP client (used in tests).
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.httpClient = hc
}

// Info calls GET / and returns cluster info with flavor detection.
func (c *Client) Info(ctx context.Context) (ClusterInfo, error) {
	body, err := c.doGet(ctx, "/")
	if err != nil {
		return ClusterInfo{}, fmt.Errorf("cluster info: %w", err)
	}

	var raw struct {
		Name    string `json:"name"`
		Version struct {
			Number       string `json:"number"`
			Distribution string `json:"distribution"`
		} `json:"version"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ClusterInfo{}, fmt.Errorf("parsing cluster info: %w", err)
	}

	flavor := Elasticsearch
	if strings.EqualFold(raw.Version.Distribution, "opensearch") {
		flavor = OpenSearch
	}

	return ClusterInfo{
		Name:    raw.Name,
		Flavor:  flavor,
		Version: raw.Version.Number,
	}, nil
}

// doGet performs a GET request with auth headers and returns the response body.
func (c *Client) doGet(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	} else if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// resolveCloudID decodes an Elastic Cloud ID.
// Format: name:base64(host$es_id$kibana_id)
func resolveCloudID(cloudID string) (string, error) {
	parts := strings.SplitN(cloudID, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", fmt.Errorf("invalid cloud ID format: missing colon separator")
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		// Try URL-safe encoding without padding
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return "", fmt.Errorf("decoding cloud ID: %w", err)
		}
	}

	segments := strings.SplitN(string(decoded), "$", 3)
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return "", fmt.Errorf("invalid cloud ID payload: expected host$es_id[$kibana_id]")
	}

	return fmt.Sprintf("https://%s.%s:443", segments[1], segments[0]), nil
}
