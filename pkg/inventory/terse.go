package inventory

import (
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

const terseDescriptionHint = " (use gh_tool_help for full docs)"

func terseTools(tools []ServerTool) []ServerTool {
	result := make([]ServerTool, 0, len(tools))
	for i := range tools {
		tool := tools[i]
		toolCopy := tool
		toolDefCopy := tool.Tool
		toolDefCopy.Description = shortenToolDescription(tool.Tool.Description)
		toolDefCopy.InputSchema = terseInputSchema(tool.Tool.InputSchema)
		toolCopy.Tool = toolDefCopy
		result = append(result, toolCopy)
	}
	return result
}

func terseInputSchema(schema any) any {
	s, ok := schema.(*jsonschema.Schema)
	if !ok || s == nil {
		return schema
	}
	cloned := s.CloneSchemas()
	if cloned == nil || cloned.Properties == nil {
		return cloned
	}
	for key, prop := range cloned.Properties {
		if prop == nil {
			continue
		}
		propCopy := *prop
		propCopy.Description = shortenParamDescription(prop.Description)
		cloned.Properties[key] = &propCopy
	}
	return cloned
}

func shortenToolDescription(s string) string {
	short := firstSentence(s, 120)
	if short == "" {
		return strings.TrimSpace(terseDescriptionHint)
	}
	return short + terseDescriptionHint
}

func shortenParamDescription(s string) string {
	return firstSentence(s, 80)
}

func firstSentence(s string, capLimit int) string {
	if capLimit <= 0 {
		return ""
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	end := len(runes)
	for i, r := range runes {
		if r == '\n' {
			end = min(end, i)
			break
		}
	}
	for i := 0; i+1 < len(runes); i++ {
		if runes[i] == '.' && runes[i+1] == ' ' {
			end = min(end, i+1)
			break
		}
	}
	end = min(end, capLimit)
	out := strings.TrimSpace(string(runes[:end]))
	out = strings.TrimRight(out, " \t\r\n.,;:!?-")
	return out
}
