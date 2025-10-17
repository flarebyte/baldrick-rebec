package opensearch

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "strings"
)

const (
    messagesContentIndex = "messages_content"
)

type MessageContent struct {
    MessageID   string                 `json:"message_id"`
    Content     string                 `json:"content"`
    ContentType string                 `json:"content_type,omitempty"`
    Language    string                 `json:"language,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CanonicalizeBody normalizes a message body for hashing/deduplication.
func CanonicalizeBody(body string) string {
    // Normalize line endings to \n and trim whitespace at ends.
    s := strings.ReplaceAll(body, "\r\n", "\n")
    s = strings.ReplaceAll(s, "\r", "\n")
    s = strings.TrimSpace(s)
    // Optionally: collapse multiple blank lines, trim trailing spaces per line.
    lines := strings.Split(s, "\n")
    for i := range lines {
        lines[i] = strings.TrimRight(lines[i], " \t")
    }
    return strings.Join(lines, "\n")
}

// HashBodySHA256 returns the hex-encoded SHA256 of the canonicalized body.
func HashBodySHA256(body string) string {
    canon := CanonicalizeBody(body)
    sum := sha256.Sum256([]byte(canon))
    return hex.EncodeToString(sum[:])
}

// EnsureMessagesContentIndex creates the index with basic mappings/settings if it doesn't exist.
func (c *Client) EnsureMessagesContentIndex(ctx context.Context) error {
    // HEAD /{index}
    req, _ := http.NewRequest(http.MethodHead, c.baseURL+"/"+messagesContentIndex, nil)
    resp, err := c.do(ctx, req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusOK {
        return nil
    }
    if resp.StatusCode != http.StatusNotFound {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("check index: status=%d body=%s", resp.StatusCode, string(b))
    }

    // Create index
    payload := map[string]interface{}{
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "message_id":   map[string]string{"type": "keyword"},
                "content":       map[string]string{"type": "text"},
                "content_type":  map[string]string{"type": "keyword"},
                "language":      map[string]string{"type": "keyword"},
                "metadata":      map[string]interface{}{"type": "object", "enabled": true},
            },
        },
        "settings": map[string]interface{}{
            "index.lifecycle.name": "messages-content-ilm",
            "refresh_interval":     "1s",
        },
    }
    buf, _ := json.Marshal(payload)
    creq, _ := http.NewRequest(http.MethodPut, c.baseURL+"/"+messagesContentIndex, bytes.NewReader(buf))
    creq.Header.Set("Content-Type", "application/json")
    cresp, err := c.do(ctx, creq)
    if err != nil {
        return err
    }
    defer cresp.Body.Close()
    if cresp.StatusCode >= 300 {
        b, _ := io.ReadAll(cresp.Body)
        return fmt.Errorf("create index: status=%d body=%s", cresp.StatusCode, string(b))
    }
    return nil
}

// PutMessageContent indexes a message content document. Returns the computed ID.
func (c *Client) PutMessageContent(ctx context.Context, content, contentType, language string, metadata map[string]interface{}) (string, error) {
    id := HashBodySHA256(content)
    doc := MessageContent{
        MessageID:   id,
        Content:     content,
        ContentType: contentType,
        Language:    language,
        Metadata:    metadata,
    }
    body, _ := json.Marshal(doc)
    // PUT /{index}/_doc/{id}
    url := fmt.Sprintf("%s/%s/_doc/%s", c.baseURL, messagesContentIndex, id)
    req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.do(ctx, req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("index doc: status=%d body=%s", resp.StatusCode, string(b))
    }
    return id, nil
}

// GetMessageContent fetches a message content by its ID.
func (c *Client) GetMessageContent(ctx context.Context, id string) (MessageContent, error) {
    var out MessageContent
    if strings.TrimSpace(id) == "" {
        return out, errors.New("empty id")
    }
    // GET /{index}/_doc/{id}
    url := fmt.Sprintf("%s/%s/_doc/%s", c.baseURL, messagesContentIndex, id)
    req, _ := http.NewRequest(http.MethodGet, url, nil)
    resp, err := c.do(ctx, req)
    if err != nil {
        return out, err
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusNotFound {
        return out, fmt.Errorf("not found: %s", id)
    }
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        return out, fmt.Errorf("get doc: status=%d body=%s", resp.StatusCode, string(b))
    }
    // Parse minimal subset: { _source: { ... } }
    var envelope struct {
        Source MessageContent `json:"_source"`
    }
    dec := json.NewDecoder(resp.Body)
    if err := dec.Decode(&envelope); err != nil {
        return out, err
    }
    // For convenience, ensure MessageID is set
    if envelope.Source.MessageID == "" {
        envelope.Source.MessageID = id
    }
    return envelope.Source, nil
}

