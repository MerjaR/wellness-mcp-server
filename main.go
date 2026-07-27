// Command wellness-mcp-server exposes a small slice of the wellness
// platform's recipe/nutrition data as MCP tools over the streamable
// HTTP transport, so any MCP client can query it directly.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MerjaR/wellness-mcp-server/internal/db"
	"github.com/MerjaR/wellness-mcp-server/internal/tools"
)

func main() {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	database, err := db.New(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "wellness-mcp-server",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_recipes",
		Description: "Search the recipe database by free text, cuisine, meal type, dietary tags, and max prep time.",
	}, tools.NewSearchRecipesTool(database))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recipe_nutrition",
		Description: "Get full nutrition detail for a recipe by id — totals, per-serving figures, and per-ingredient breakdown.",
	}, tools.NewGetRecipeNutritionTool(database))

	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless: true, // no session state needed for stateless data lookups
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	log.Printf("MCP server listening on %s (endpoint: /mcp)", addr)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}