package github

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolHelpResponse struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Parameters  []inventory.ParamInfo `json:"parameters"`
}

func ToolHelp(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataMeta,
		mcp.Tool{
			Name:        "gh_tool_help",
			Description: t("TOOL_GH_TOOL_HELP_DESCRIPTION", "Get full descriptions and parameter docs for a specific GitHub MCP tool."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GH_TOOL_HELP_TITLE", "Get full tool documentation"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type:     "object",
				Required: []string{"tool_name"},
				Properties: map[string]*jsonschema.Schema{
					"tool_name": {
						Type:        "string",
						Description: t("TOOL_GH_TOOL_HELP_TOOL_NAME_DESCRIPTION", "Name of the GitHub MCP tool to retrieve full documentation for."),
					},
				},
			},
		},
		[]scopes.Scope{},
		func(ctx context.Context, _ ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			toolName, err := RequiredParam[string](args, "tool_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			inv, ok := InventoryFromContext(ctx)
			if !ok || inv == nil {
				return utils.NewToolResultError("inventory not available in request context"), nil, nil
			}

			info, found := inv.ToolVerboseInfo(toolName)
			if !found {
				valid := validToolNames(ctx, inv)
				return utils.NewToolResultError(fmt.Sprintf("unknown tool_name %q. valid tool names: %s", toolName, strings.Join(valid, ", "))), nil, nil
			}

			return MarshalledTextResult(toolHelpResponse{
				Name:        info.Name,
				Description: info.Description,
				Parameters:  info.Parameters,
			}), nil, nil
		},
	)
}

func validToolNames(ctx context.Context, inv *inventory.Inventory) []string {
	tools := inv.ToolsForRegistration(ctx)
	names := make([]string, 0, len(tools))
	seen := map[string]struct{}{}
	for _, tool := range tools {
		if _, ok := seen[tool.Tool.Name]; ok {
			continue
		}
		seen[tool.Tool.Name] = struct{}{}
		names = append(names, tool.Tool.Name)
	}
	slices.Sort(names)
	return names
}
