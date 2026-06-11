package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSpillEnvelope struct {
	Spilled  bool   `json:"spilled"`
	Tool     string `json:"tool"`
	Path     string `json:"path"`
	Bytes    int    `json:"bytes"`
	MIMEHint string `json:"mime_hint"`
	Head     string `json:"head"`
	Tail     string `json:"tail"`
	Hint     string `json:"hint"`
}

func TestNewToolResultTextPassThroughUnderThreshold(t *testing.T) {
	spillDir := t.TempDir()
	SetSpillConfig(SpillConfig{
		Dir:       spillDir,
		Threshold: 1024,
		HeadBytes: 8,
		TailBytes: 8,
	})
	defer SetSpillConfig(SpillConfig{})

	result := NewToolResultText("small response")
	text := getTextResult(t, result)

	assert.Equal(t, "small response", text.Text)

	entries, err := os.ReadDir(spillDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestNewToolResultTextPassThroughWhenSpillDisabled(t *testing.T) {
	SetSpillConfig(SpillConfig{
		Threshold: 1,
		HeadBytes: 8,
		TailBytes: 8,
	})
	defer SetSpillConfig(SpillConfig{})

	message := strings.Repeat("x", 32)
	result := NewToolResultText(message)
	text := getTextResult(t, result)

	assert.Equal(t, message, text.Text)
}

func TestNewToolResultTextSpillsOversizedResponse(t *testing.T) {
	spillDir := filepath.Join(t.TempDir(), "spill")
	message := "{\"emoji\":\"😀😀😀😀😀\",\"text\":\"abcdefghijklmnopqrstuvwxyz\"}"
	body := []byte(message)

	SetSpillConfig(SpillConfig{
		Dir:       spillDir,
		Threshold: 12,
		HeadBytes: 13,
		TailBytes: 11,
	})
	defer SetSpillConfig(SpillConfig{})

	result := NewToolResultText(message)
	text := getTextResult(t, result)
	envelope := decodeSpillEnvelope(t, text.Text)

	assert.True(t, envelope.Spilled)
	assert.Equal(t, "tool-result", envelope.Tool)
	assert.Equal(t, len(body), envelope.Bytes)
	assert.Equal(t, "application/json", envelope.MIMEHint)
	assert.Equal(t, strings.ToValidUTF8(string(body[:13]), ""), envelope.Head)
	assert.Equal(t, strings.ToValidUTF8(string(body[len(body)-11:]), ""), envelope.Tail)
	assert.True(t, utf8.ValidString(envelope.Head))
	assert.True(t, utf8.ValidString(envelope.Tail))
	assert.Contains(t, envelope.Hint, envelope.Path)
	assert.True(t, filepath.IsAbs(envelope.Path))
	assert.Equal(t, ".json", filepath.Ext(envelope.Path))

	spilledBody, err := os.ReadFile(envelope.Path)
	require.NoError(t, err)
	assert.Equal(t, message, string(spilledBody))

	dirInfo, err := os.Stat(spillDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), dirInfo.Mode().Perm())

	fileInfo, err := os.Stat(envelope.Path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), fileInfo.Mode().Perm())
}

func TestNewToolResultTextSpillFilenameIsUnique(t *testing.T) {
	SetSpillConfig(SpillConfig{
		Dir:       t.TempDir(),
		Threshold: 1,
		HeadBytes: 8,
		TailBytes: 8,
	})
	defer SetSpillConfig(SpillConfig{})

	first := decodeSpillEnvelope(t, getTextResult(t, NewToolResultText("first spill")).Text)
	second := decodeSpillEnvelope(t, getTextResult(t, NewToolResultText("second spill")).Text)

	assert.NotEqual(t, first.Path, second.Path)
}

func getTextResult(t *testing.T, result *mcp.CallToolResult) *mcp.TextContent {
	t.Helper()
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)

	return text
}

func decodeSpillEnvelope(t *testing.T, text string) testSpillEnvelope {
	t.Helper()

	var envelope testSpillEnvelope
	require.NoError(t, json.Unmarshal([]byte(text), &envelope))

	return envelope
}
