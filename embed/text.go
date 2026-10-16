package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/thomas-elliott/go-graph/graph"
)

// Text is the embedding content joining destination, heading or name, and body text.
func Text(n graph.Node) string {
	var parts []string
	for _, key := range []string{"dest", "heading", "name", "text", "content"} {
		if s, ok := n.Properties[key].(string); ok && s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}

// Hash is a SHA256 hash of the embedding content.
func Hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
