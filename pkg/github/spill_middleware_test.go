package github

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handlerReturning returns a MethodHandler test double that returns the given result.
func handlerReturning(result mcp.Result) mcp.MethodHandler {
	return func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return result, nil
	}
}

// callToolRequest returns a *mcp.CallToolRequest with the given tool name.
func callToolRequest(name string) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: name},
	}
}

func TestSpillToolNameMiddlewarePassesThroughNonSpillText(t *testing.T) {
	plainResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "plain text response"},
		},
	}

	handler := SpillToolNameMiddleware(handlerReturning(plainResult))
	got, err := handler(context.Background(), "tools/call", callToolRequest("my_tool"))

	require.NoError(t, err)
	callResult, ok := got.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, callResult.Content, 1)
	text := callResult.Content[0].(*mcp.TextContent)
	assert.Equal(t, "plain text response", text.Text)
}

func TestSpillToolNameMiddlewareRewritesToolField(t *testing.T) {
	const toolName = "get_file_contents"

	spillDir := t.TempDir()
	utils.SetSpillConfig(utils.SpillConfig{
		Dir:       spillDir,
		Threshold: 10,
		HeadBytes: 8,
		TailBytes: 8,
	})
	defer utils.SetSpillConfig(utils.SpillConfig{})

	// Produce a real spill envelope via utils.NewToolResultText.
	largeBody := strings.Repeat("x", 100)
	spillResult := utils.NewToolResultText(largeBody)
	require.Len(t, spillResult.Content, 1)
	text := spillResult.Content[0].(*mcp.TextContent)
	require.True(t, strings.HasPrefix(text.Text, spillEnvelopePrefix), "expected spill envelope, got: %s", text.Text)

	handler := SpillToolNameMiddleware(handlerReturning(spillResult))
	got, err := handler(context.Background(), "tools/call", callToolRequest(toolName))

	require.NoError(t, err)
	callResult, ok := got.(*mcp.CallToolResult)
	require.True(t, ok)
	require.Len(t, callResult.Content, 1)

	var envelope map[string]any
	outText := callResult.Content[0].(*mcp.TextContent)
	require.NoError(t, json.Unmarshal([]byte(outText.Text), &envelope))
	assert.Equal(t, toolName, envelope["tool"])
}

func TestSpillToolNameMiddlewarePassesThroughNonCallToolRequest(t *testing.T) {
	// Use a ListToolsRequest, which is not a *CallToolRequest.
	listReq := &mcp.ListToolsRequest{}

	spillText := spillEnvelopePrefix + `"tool":"tool-result","path":"/tmp/f","bytes":1,"mime_hint":"text/plain","head":"","tail":"","hint":""}`
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: spillText},
		},
	}

	handler := SpillToolNameMiddleware(handlerReturning(result))
	got, err := handler(context.Background(), "tools/list", listReq)

	require.NoError(t, err)
	callResult, ok := got.(*mcp.CallToolResult)
	require.True(t, ok)
	// Tool field must not have been rewritten.
	var envelope map[string]any
	outText := callResult.Content[0].(*mcp.TextContent)
	require.NoError(t, json.Unmarshal([]byte(outText.Text), &envelope))
	assert.Equal(t, "tool-result", envelope["tool"])
}

func TestSpillToolNameMiddlewarePassesThroughMangledEnvelope(t *testing.T) {
	mangled := `{"spilled":true,bogus}`
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: mangled},
		},
	}

	handler := SpillToolNameMiddleware(handlerReturning(result))
	got, err := handler(context.Background(), "tools/call", callToolRequest("my_tool"))

	require.NoError(t, err)
	callResult, ok := got.(*mcp.CallToolResult)
	require.True(t, ok)
	outText := callResult.Content[0].(*mcp.TextContent)
	// Mangled JSON must pass through unchanged.
	assert.Equal(t, mangled, outText.Text)
}

func TestSpillToolNameMiddlewarePassesThroughMultipleContent(t *testing.T) {
	spillText := spillEnvelopePrefix + `"tool":"tool-result","path":"/tmp/f","bytes":1,"mime_hint":"text/plain","head":"","tail":"","hint":""}`
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: spillText},
			&mcp.TextContent{Text: "extra"},
		},
	}

	handler := SpillToolNameMiddleware(handlerReturning(result))
	got, err := handler(context.Background(), "tools/call", callToolRequest("my_tool"))

	require.NoError(t, err)
	callResult, ok := got.(*mcp.CallToolResult)
	require.True(t, ok)
	// Two content items — must pass through unchanged.
	require.Len(t, callResult.Content, 2)
	outText := callResult.Content[0].(*mcp.TextContent)
	assert.Equal(t, spillText, outText.Text)
}
