package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Client calls the embeddings endpoint using OpenAI style API.
type Client struct {
	URL    string
	Model  string
	Dims   int
	APIKey string
}

// FromEnv reads embedding config from the .env file.
func FromEnv() (*Client, error) {
	c := &Client{
		URL:    strings.TrimSuffix(os.Getenv("EMBED_URL"), "/"),
		Model:  os.Getenv("EMBED_MODEL"),
		APIKey: os.Getenv("EMBED_API_KEY"),
	}
	if c.URL == "" || c.Model == "" {
		return nil, fmt.Errorf("EMBED_URL and EMBED_MODEL must be set")
	}
	dims, err := strconv.Atoi(os.Getenv("EMBED_DIMS"))
	if err != nil || dims <= 0 {
		return nil, fmt.Errorf("EMBED_DIMS must be a positive number")
	}
	c.Dims = dims
	return c, nil
}

type request struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions"`
}

type response struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed returns one vector per text.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(request{Model: c.Model, Input: texts, Dimensions: c.Dims})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))
		return nil, fmt.Errorf("embed: %s: %s", resp.Status, msg)
	}

	var out response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("embed: decode: %w", err)
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embed: sent %d texts, got %d vectors", len(texts), len(out.Data))
	}

	// It could return out of order, so reorder by index.
	vectors := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			return nil, fmt.Errorf("embed: vector index %d out of range", d.Index)
		}
		if len(d.Embedding) != c.Dims {
			return nil, fmt.Errorf("embed: asked for %d dimensions, got %d", c.Dims, len(d.Embedding))
		}
		vectors[d.Index] = d.Embedding
	}
	return vectors, nil
}
