package github

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/require"
)

func TestToolHelpKnownToolReturnsVerboseInfo(t *testing.T) {
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{"issues"}).
		Build()
	require.NoError(t, err)

	req := createMCPRequest(map[string]any{"tool_name": "issue_read"})
	ctx := ContextWithInventory(ContextWithDeps(context.Background(), stubDeps{obsv: stubExporters()}), inv)
	tool := ToolHelp(translations.NullTranslationHelper)
	result, err := tool.Handler(nil)(ctx, &req)
	require.NoError(t, err)
	require.False(t, result.IsError)

	info := toolHelpResponse{}
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &info))
	require.Equal(t, "issue_read", info.Name)
	require.NotEmpty(t, info.Description)
	require.Contains(t, strings.ToLower(info.Description), "issue")
	require.NotEmpty(t, info.Parameters)

	var foundMethod bool
	var methodEnum []any
	var methodDescription string
	for i := range info.Parameters {
		if info.Parameters[i].Name == "method" {
			methodEnum = info.Parameters[i].Enum
			methodDescription = info.Parameters[i].Description
			foundMethod = true
			break
		}
	}
	require.True(t, foundMethod)
	require.NotEmpty(t, methodEnum, "method enum should be preserved")
	require.NotEmpty(t, methodDescription)
}

func TestToolHelpUnknownToolReturnsErrorWithSuggestions(t *testing.T) {
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{"issues"}).
		Build()
	require.NoError(t, err)

	req := createMCPRequest(map[string]any{"tool_name": "not_a_tool"})
	ctx := ContextWithInventory(ContextWithDeps(context.Background(), stubDeps{obsv: stubExporters()}), inv)
	tool := ToolHelp(translations.NullTranslationHelper)
	result, err := tool.Handler(nil)(ctx, &req)
	require.NoError(t, err)
	require.True(t, result.IsError)

	msg := getErrorResult(t, result).Text
	require.Contains(t, msg, "valid tool names")
	require.Contains(t, msg, "issue_read")
	require.Contains(t, msg, "gh_tool_help")
}

func TestToolHelpWorksWhenTerseModeOff(t *testing.T) {
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{"repos"}).
		Build()
	require.NoError(t, err)

	req := createMCPRequest(map[string]any{"tool_name": "list_branches"})
	ctx := ContextWithInventory(ContextWithDeps(context.Background(), stubDeps{obsv: stubExporters()}), inv)
	tool := ToolHelp(translations.NullTranslationHelper)
	result, err := tool.Handler(nil)(ctx, &req)
	require.NoError(t, err)
	require.False(t, result.IsError)

	info := toolHelpResponse{}
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &info))
	require.Equal(t, "list_branches", info.Name)
	require.NotEmpty(t, info.Description)
}
