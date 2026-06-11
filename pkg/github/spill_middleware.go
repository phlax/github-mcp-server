package github

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// spillEnvelopePrefix is the cheap discriminator used to avoid JSON-parsing
// every text response. The envelope is produced by utils.NewToolResultText and
// always starts with this prefix because encoding/json preserves struct field
// order and Spilled is the first field of spillEnvelope (in pkg/utils/result.go).
const spillEnvelopePrefix = `{"spilled":true,`

// SpillToolNameMiddleware rewrites the "tool" field inside spill envelopes
// produced by pkg/utils so it reflects the actual tool name from the
// CallToolRequest. The spill writer in pkg/utils has no access to the tool
// name, so this middleware injects it after the fact.
//
// Non-CallTool methods, non-envelope results, and unparseable envelopes are
// all passed through unchanged.
func SpillToolNameMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, req)
		if err != nil {
			return result, err
		}
		callReq, ok := req.(*mcp.CallToolRequest)
		if !ok || callReq == nil || callReq.Params == nil || callReq.Params.Name == "" {
			return result, nil
		}
		callResult, ok := result.(*mcp.CallToolResult)
		if !ok || callResult == nil || len(callResult.Content) != 1 {
			return result, nil
		}
		text, ok := callResult.Content[0].(*mcp.TextContent)
		if !ok || !strings.HasPrefix(text.Text, spillEnvelopePrefix) {
			return result, nil
		}

		var envelope map[string]any
		if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
			return result, nil
		}
		if _, hasSpilled := envelope["spilled"].(bool); !hasSpilled {
			return result, nil
		}
		envelope["tool"] = callReq.Params.Name

		rewritten, err := json.Marshal(envelope)
		if err != nil {
			return result, nil
		}
		text.Text = string(rewritten)
		return result, nil
	}
}
