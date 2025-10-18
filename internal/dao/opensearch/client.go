package opensearch

import (
    "context"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/flarebyte/baldrick-rebec/internal/config"
)

type Client struct {
    httpClient *http.Client
    baseURL    string // e.g. http://127.0.0.1:9200 or https://127.0.0.1:9200
    username   string
    password   string
}

// NewClientFromConfigApp builds a client using the app role credentials.
func NewClientFromConfigApp(cfg config.Config) *Client {
    scheme := cfg.OpenSearch.Scheme
    if scheme == "" {
        scheme = "http"
    }
    host := cfg.OpenSearch.Host
    if host == "" {
        host = "127.0.0.1"
    }
    port := cfg.OpenSearch.Port
    if port == 0 {
        port = config.DefaultOpenSearchPort
    }
    baseURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)

    tr := &http.Transport{}
    if scheme == "https" && cfg.OpenSearch.InsecureSkipVerify {
        tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // dev-only
    }
    hc := &http.Client{Transport: tr, Timeout: 30 * time.Second}
    return &Client{
        httpClient: hc,
        baseURL:    baseURL,
        username:   firstNonEmpty(cfg.OpenSearch.App.Username, cfg.OpenSearch.Admin.Username),
        password:   firstNonEmpty(cfg.OpenSearch.App.Password, cfg.OpenSearch.App.Password, cfg.OpenSearch.Admin.Password, cfg.OpenSearch.Admin.PasswordTemp),
    }
}

// NewClientFromConfigAdmin builds a client using the admin role credentials.
func NewClientFromConfigAdmin(cfg config.Config) *Client {
    scheme := cfg.OpenSearch.Scheme
    if scheme == "" {
        scheme = "http"
    }
    host := cfg.OpenSearch.Host
    if host == "" {
        host = "127.0.0.1"
    }
    port := cfg.OpenSearch.Port
    if port == 0 {
        port = config.DefaultOpenSearchPort
    }
    baseURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)

    tr := &http.Transport{}
    if scheme == "https" && cfg.OpenSearch.InsecureSkipVerify {
        tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
    }
    hc := &http.Client{Transport: tr, Timeout: 30 * time.Second}
    return &Client{
        httpClient: hc,
        baseURL:    baseURL,
        username:   firstNonEmpty(cfg.OpenSearch.Admin.Username, cfg.OpenSearch.App.Username),
        password:   firstNonEmpty(cfg.OpenSearch.Admin.Password, cfg.OpenSearch.Admin.PasswordTemp, cfg.OpenSearch.App.Password),
    }
}

// Backward-compat constructor: use app, then legacy fields if present.
func NewClientFromConfig(cfg config.Config) *Client {
    c := NewClientFromConfigApp(cfg)
    // If neither admin nor app is configured but legacy exists, set from legacy
    if c.username == "" && cfg.OpenSearch.Admin.Username == "" && cfg.OpenSearch.App.Username == "" {
        c.username = cfg.OpenSearch.Username
        c.password = cfg.OpenSearch.Password
    }
    return c
}

func firstNonEmpty(values ...string) string {
    for _, v := range values {
        if v != "" {
            return v
        }
    }
    return ""
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
    req = req.WithContext(ctx)
    if c.username != "" {
        req.SetBasicAuth(c.username, c.password)
    }
    return c.httpClient.Do(req)
}

// ClusterHealth returns the cluster health status string (e.g., green, yellow, red).
func (c *Client) ClusterHealth(ctx context.Context) (string, error) {
    req, _ := http.NewRequest(http.MethodGet, c.baseURL+"/_cluster/health", nil)
    resp, err := c.do(ctx, req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("cluster health: status=%d body=%s", resp.StatusCode, string(b))
    }
    var obj struct{
        Status string `json:"status"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
        return "", err
    }
    return obj.Status, nil
}

// IndexExists checks if a given index exists.
func (c *Client) IndexExists(ctx context.Context, index string) (bool, error) {
    req, _ := http.NewRequest(http.MethodHead, c.baseURL+"/"+index, nil)
    resp, err := c.do(ctx, req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusOK { return true, nil }
    if resp.StatusCode == http.StatusNotFound { return false, nil }
    b, _ := io.ReadAll(resp.Body)
    return false, fmt.Errorf("index exists check: status=%d body=%s", resp.StatusCode, string(b))
}
