package mcpserver

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewHandler builds the MCP server with all tools registered and
// returns a plain http.Handler serving the streamable HTTP transport.
//
// This is designed to be mountable two ways:
//  1. Standalone — served directly at the root of its own listener
//     (see cmd/server/main.go in this repo).
//  2. Embedded — mounted at a chosen path (e.g. "/mcp") inside another
//     Go service's existing router, sharing that service's deployment.
//     For example, with Gin: router.Any("/mcp", gin.WrapH(handler)).
//
// The pool is not owned by the handler — the caller is responsible
// for opening and closing it. This lets an embedding service reuse
// its own existing connection pool rather than opening a second one.
func NewHandler(pool *pgxpool.Pool) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "wellness-mcp-server",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_recipes",
		Description: "Search the recipe database by free text, cuisine, meal type, dietary tags, and max prep time.",
	}, NewSearchRecipesTool(pool))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recipe_nutrition",
		Description: "Get full nutrition detail for a recipe by id — totals, per-serving figures, and per-ingredient breakdown.",
	}, NewGetRecipeNutritionTool(pool))

	return mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless: true, // no session state needed for stateless data lookups

		// The SDK's default DNS-rebinding protection rejects requests
		// whose Host header doesn't match "localhost" whenever the
		// underlying TCP connection looks like a loopback connection.
		// That's meant to protect servers actually running on someone's
		// own machine — but PaaS platforms like Render terminate the
		// public request at their edge and forward it to the container
		// over an internal/loopback-like connection while preserving the
		// real public Host header, which trips the same check for a
		// legitimately deployed, internet-facing service. Safe to
		// disable here since this server is meant to be reached over
		// the public internet, not just from localhost.
		DisableLocalhostProtection: true,
	})
}