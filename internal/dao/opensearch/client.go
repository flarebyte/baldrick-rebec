package opensearch

import (
    "context"
    "crypto/tls"
    "fmt"
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

// NewClientFromConfig builds a simple HTTP client using the OpenSearch config.
func NewClientFromConfig(cfg config.Config) *Client {
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
        username:   cfg.OpenSearch.Username,
        password:   cfg.OpenSearch.Password,
    }
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
    req = req.WithContext(ctx)
    if c.username != "" {
        req.SetBasicAuth(c.username, c.password)
    }
    return c.httpClient.Do(req)
}

