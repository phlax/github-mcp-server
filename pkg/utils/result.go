package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SpillConfig controls optional spilling of oversized tool results to disk.
type SpillConfig struct {
	Dir       string
	Threshold int
	HeadBytes int
	TailBytes int
}

// ResultMeta carries optional metadata for tool result construction.
type ResultMeta struct {
	Tool     string
	MIMEHint string
}

type spillEnvelope struct {
	Spilled  bool   `json:"spilled"`
	Tool     string `json:"tool"`
	Path     string `json:"path"`
	Bytes    int    `json:"bytes"`
	MIMEHint string `json:"mime_hint"`
	Head     string `json:"head"`
	Tail     string `json:"tail"`
	Hint     string `json:"hint"`
}

var (
	spillConfigMu sync.RWMutex
	spillConfig   SpillConfig
)

// SetSpillConfig updates the process-global oversized response spill configuration.
func SetSpillConfig(cfg SpillConfig) {
	spillConfigMu.Lock()
	defer spillConfigMu.Unlock()
	spillConfig = cfg
}

func NewToolResultText(message string) *mcp.CallToolResult {
	return NewToolResultTextWithMeta(message, ResultMeta{})
}

// NewToolResultTextWithMeta creates a text result and optionally spills oversized bodies to disk.
func NewToolResultTextWithMeta(message string, meta ResultMeta) *mcp.CallToolResult {
	cfg := getSpillConfig()
	if !shouldSpill(message, cfg) {
		return newInlineToolResultText(message)
	}

	result, err := spillToolResult(message, meta, cfg)
	if err == nil {
		return result
	}

	slog.Default().Error("failed to spill oversized tool result", "error", err, "tool", meta.Tool, "spill_dir", cfg.Dir)

	return newInlineToolResultText(message)
}

func newInlineToolResultText(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
		},
	}
}

func getSpillConfig() SpillConfig {
	spillConfigMu.RLock()
	defer spillConfigMu.RUnlock()
	return spillConfig
}

func shouldSpill(message string, cfg SpillConfig) bool {
	return cfg.Dir != "" && len(message) > cfg.Threshold
}

func spillToolResult(message string, meta ResultMeta, cfg SpillConfig) (*mcp.CallToolResult, error) {
	spillDir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve spill dir: %w", err)
	}

	if err := os.MkdirAll(spillDir, 0o755); err != nil {
		return nil, fmt.Errorf("create spill dir: %w", err)
	}

	body := []byte(message)
	toolName := spillToolName(meta.Tool)
	mimeHint := meta.MIMEHint
	ext := ".txt"
	if json.Valid(body) {
		mimeHint = "application/json"
		ext = ".json"
	} else if mimeHint == "" {
		mimeHint = "text/plain"
	}

	filename, err := spillFilename(toolName, ext)
	if err != nil {
		return nil, fmt.Errorf("generate spill filename: %w", err)
	}

	path := filepath.Join(spillDir, filename)
	//nolint:gosec // Spilled tool results must remain readable by the agent via the shared filesystem path.
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return nil, fmt.Errorf("write spill file: %w", err)
	}

	envelopeBody, err := json.Marshal(spillEnvelope{
		Spilled:  true,
		Tool:     toolName,
		Path:     path,
		Bytes:    len(body),
		MIMEHint: mimeHint,
		Head:     previewHead(body, cfg.HeadBytes),
		Tail:     previewTail(body, cfg.TailBytes),
		Hint:     fmt.Sprintf("Response too large to inline. The full body is at %q. Read it with filesystem tools (head/tail/grep/jq).", path),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal spill envelope: %w", err)
	}

	return newInlineToolResultText(string(envelopeBody)), nil
}

func spillFilename(tool, ext string) (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%d-%s%s", sanitizeSpillToolName(tool), time.Now().UnixMilli(), hex.EncodeToString(suffix[:]), ext), nil
}

func spillToolName(tool string) string {
	if tool == "" {
		return "tool-result"
	}

	return tool
}

func sanitizeSpillToolName(tool string) string {
	if tool == "" {
		return "tool-result"
	}

	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, tool)
	if safe == "" {
		return "tool-result"
	}

	return safe
}

func previewHead(body []byte, n int) string {
	if n <= 0 || len(body) == 0 {
		return ""
	}
	if n > len(body) {
		n = len(body)
	}

	return strings.ToValidUTF8(string(body[:n]), "")
}

func previewTail(body []byte, n int) string {
	if n <= 0 || len(body) == 0 {
		return ""
	}
	if n > len(body) {
		n = len(body)
	}

	return strings.ToValidUTF8(string(body[len(body)-n:]), "")
}

func NewToolResultError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
		},
		IsError: true,
	}
}

func NewToolResultErrorFromErr(message string, err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message + ": " + err.Error(),
			},
		},
		IsError: true,
	}
}

func NewToolResultResource(message string, contents *mcp.ResourceContents) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
			&mcp.EmbeddedResource{
				Resource: contents,
			},
		},
		IsError: false,
	}
}

func NewToolResultResourceLink(message string, link *mcp.ResourceLink) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: message,
			},
			link,
		},
		IsError: false,
	}
}
