// Command wellness-mcp-server exposes a small slice of the wellness
// platform's recipe/nutrition data as MCP tools over the streamable
// HTTP transport, so any MCP client can query it directly.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/MerjaR/wellness-mcp-server/internal/db"
	"github.com/MerjaR/wellness-mcp-server/mcpserver"
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

	handler := mcpserver.NewHandler(database.Pool)

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