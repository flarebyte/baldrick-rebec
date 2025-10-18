package opensearch

import (
    "bytes"
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

// EnsureILMPolicy ensures an ILM policy exists; if force is true, updates/overwrites.
func (c *Client) EnsureILMPolicy(ctx context.Context, name string, policy map[string]interface{}, force bool) error {
    if name == "" { return fmt.Errorf("empty ILM policy name") }
    // Check existence
    req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/_ilm/policy/%s", c.baseURL, name), nil)
    resp, err := c.do(ctx, req)
    if err != nil { return err }
    resp.Body.Close()
    exists := resp.StatusCode == http.StatusOK
    if exists && !force {
        return nil
    }
    body, err := json.Marshal(policy)
    if err != nil { return err }
    putReq, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/_ilm/policy/%s", c.baseURL, name), bytes.NewReader(body))
    putReq.Header.Set("Content-Type", "application/json")
    putResp, err := c.do(ctx, putReq)
    if err != nil { return err }
    defer putResp.Body.Close()
    if putResp.StatusCode >= 300 {
        b, _ := io.ReadAll(putResp.Body)
        return fmt.Errorf("ensure ILM policy: status=%d body=%s", putResp.StatusCode, string(b))
    }
    return nil
}

// AttachILMToIndex attaches a policy to an index via settings update.
func (c *Client) AttachILMToIndex(ctx context.Context, index, policyName string) error {
    if index == "" || policyName == "" {
        return fmt.Errorf("empty index or policy name")
    }
    payload := map[string]interface{}{
        "index": map[string]interface{}{
            "lifecycle": map[string]interface{}{
                "name": policyName,
            },
        },
    }
    body, _ := json.Marshal(payload)
    req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s/_settings", c.baseURL, index), bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.do(ctx, req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("attach ILM to index: status=%d body=%s", resp.StatusCode, string(b))
    }
    return nil
}

// GetILMPolicy returns the raw ILM policy JSON for the given name.
func (c *Client) GetILMPolicy(ctx context.Context, name string) ([]byte, error) {
    if name == "" { return nil, fmt.Errorf("empty ILM policy name") }
    req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/_ilm/policy/%s", c.baseURL, name), nil)
    resp, err := c.do(ctx, req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("get ILM policy: status=%d body=%s", resp.StatusCode, string(b))
    }
    body, err := io.ReadAll(resp.Body)
    if err != nil { return nil, err }
    return body, nil
}

// DeleteILMPolicy deletes the ILM policy with the given name.
func (c *Client) DeleteILMPolicy(ctx context.Context, name string) error {
    if name == "" { return fmt.Errorf("empty ILM policy name") }
    req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/_ilm/policy/%s", c.baseURL, name), nil)
    resp, err := c.do(ctx, req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("delete ILM policy: status=%d body=%s", resp.StatusCode, string(b))
    }
    return nil
}

// ListILMPolicies returns the raw JSON of all ILM policies.
func (c *Client) ListILMPolicies(ctx context.Context) ([]byte, error) {
    req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/_ilm/policy", c.baseURL), nil)
    resp, err := c.do(ctx, req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("list ILM policies: status=%d body=%s", resp.StatusCode, string(b))
    }
    b, err := io.ReadAll(resp.Body)
    if err != nil { return nil, err }
    return b, nil
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

// IndexLifecycleName returns index.lifecycle.name setting if set, else empty.
func (c *Client) IndexLifecycleName(ctx context.Context, index string) (string, error) {
    if index == "" { return "", fmt.Errorf("empty index") }
    req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s/_settings", c.baseURL, index), nil)
    resp, err := c.do(ctx, req)
    if err != nil { return "", err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("get index settings: status=%d body=%s", resp.StatusCode, string(b))
    }
    var m map[string]struct{ Settings map[string]map[string]map[string]interface{} `json:"settings"` }
    if err := json.NewDecoder(resp.Body).Decode(&m); err != nil { return "", err }
    for _, v := range m {
        if idx, ok := v.Settings["index"]; ok {
            if lc, ok := idx["lifecycle"]; ok {
                if name, ok := lc["name"].(string); ok {
                    return name, nil
                }
            }
        }
    }
    return "", nil
}

// IndexDocCount returns document count via _count API.
func (c *Client) IndexDocCount(ctx context.Context, index string) (int64, error) {
    if index == "" { return 0, fmt.Errorf("empty index") }
    req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s/_count", c.baseURL, index), nil)
    resp, err := c.do(ctx, req)
    if err != nil { return 0, err }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return 0, fmt.Errorf("get index count: status=%d body=%s", resp.StatusCode, string(b))
    }
    var obj struct{ Count int64 `json:"count"` }
    if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil { return 0, err }
    return obj.Count, nil
}
