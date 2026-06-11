package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type inventoryContextKey struct{}

func InjectInventoryMiddleware(inv *inventory.Inventory) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			ctx = ContextWithInventory(ctx, inv)
			return next(ctx, method, req)
		}
	}
}

func ContextWithInventory(ctx context.Context, inv *inventory.Inventory) context.Context {
	return context.WithValue(ctx, inventoryContextKey{}, inv)
}

func InventoryFromContext(ctx context.Context) (*inventory.Inventory, bool) {
	inv, ok := ctx.Value(inventoryContextKey{}).(*inventory.Inventory)
	return inv, ok
}
