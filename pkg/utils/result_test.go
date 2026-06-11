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
		// Intentionally split multi-byte emoji boundaries to verify head/tail previews
		// are repaired into valid UTF-8 with strings.ToValidUTF8.
		HeadBytes: 13,
		TailBytes: 11,
	})
	defer SetSpillConfig(SpillConfig{})

	result := NewToolResultText(message)
	text := getTextResult(t, result)
	envelope := decodeSpillEnvelope(t, text.Text)

	assert.True(t, envelope.bool(t, "spilled"))
	assert.Equal(t, "tool-result", envelope.string(t, "tool"))
	assert.Equal(t, len(body), envelope.int(t, "bytes"))
	assert.Equal(t, "application/json", envelope.string(t, "mime_hint"))
	assert.Equal(t, strings.ToValidUTF8(string(body[:13]), ""), envelope.string(t, "head"))
	assert.Equal(t, strings.ToValidUTF8(string(body[len(body)-11:]), ""), envelope.string(t, "tail"))
	assert.True(t, utf8.ValidString(envelope.string(t, "head")))
	assert.True(t, utf8.ValidString(envelope.string(t, "tail")))
	assert.Contains(t, envelope.string(t, "hint"), envelope.string(t, "path"))
	assert.True(t, filepath.IsAbs(envelope.string(t, "path")))
	assert.Equal(t, ".json", filepath.Ext(envelope.string(t, "path")))

	spilledBody, err := os.ReadFile(envelope.string(t, "path"))
	require.NoError(t, err)
	assert.Equal(t, message, string(spilledBody))

	dirInfo, err := os.Stat(spillDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), dirInfo.Mode().Perm())

	fileInfo, err := os.Stat(envelope.string(t, "path"))
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

	assert.NotEqual(t, first.string(t, "path"), second.string(t, "path"))
}

func getTextResult(t *testing.T, result *mcp.CallToolResult) *mcp.TextContent {
	t.Helper()
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)

	return text
}

type decodedEnvelope map[string]any

func decodeSpillEnvelope(t *testing.T, text string) decodedEnvelope {
	t.Helper()

	var envelope decodedEnvelope
	require.NoError(t, json.Unmarshal([]byte(text), &envelope))

	return envelope
}

func (e decodedEnvelope) bool(t *testing.T, key string) bool {
	t.Helper()
	value, ok := e[key].(bool)
	require.Truef(t, ok, "expected %q to be a bool, got %T", key, e[key])
	return value
}

func (e decodedEnvelope) string(t *testing.T, key string) string {
	t.Helper()
	value, ok := e[key].(string)
	require.Truef(t, ok, "expected %q to be a string, got %T", key, e[key])
	return value
}

func (e decodedEnvelope) int(t *testing.T, key string) int {
	t.Helper()
	value, ok := e[key].(float64)
	require.Truef(t, ok, "expected %q to be a number, got %T", key, e[key])
	return int(value)
}
