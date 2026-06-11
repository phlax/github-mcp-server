package inventory

import (
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
)

type ToolVerboseInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  []ParamInfo `json:"parameters"`
}

type ParamInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Enum        []any  `json:"enum,omitempty"`
}

func captureToolVerboseInfo(tools []ServerTool) map[string]ToolVerboseInfo {
	infos := make(map[string]ToolVerboseInfo, len(tools))
	for i := range tools {
		tool := tools[i]
		info := ToolVerboseInfo{
			Name:        tool.Tool.Name,
			Description: tool.Tool.Description,
		}

		schema, ok := tool.Tool.InputSchema.(*jsonschema.Schema)
		if ok && schema != nil && schema.Properties != nil {
			required := make(map[string]struct{}, len(schema.Required))
			for _, name := range schema.Required {
				required[name] = struct{}{}
			}
			for name, prop := range schema.Properties {
				if prop == nil {
					continue
				}
				_, isRequired := required[name]
				info.Parameters = append(info.Parameters, ParamInfo{
					Name:        name,
					Type:        schemaType(prop),
					Required:    isRequired,
					Description: prop.Description,
					Enum:        prop.Enum,
				})
			}
			slices.SortFunc(info.Parameters, func(a, b ParamInfo) int {
				if a.Name < b.Name {
					return -1
				}
				if a.Name > b.Name {
					return 1
				}
				return 0
			})
		}
		infos[tool.Tool.Name] = info
	}
	return infos
}

func schemaType(schema *jsonschema.Schema) string {
	if schema == nil {
		return ""
	}
	if schema.Type != "" {
		return schema.Type
	}
	if len(schema.Types) > 0 {
		return schema.Types[0]
	}
	return ""
}
