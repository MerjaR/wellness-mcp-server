# wellness-mcp-server

A small MCP (Model Context Protocol) server exposing recipe and
nutrition data as tools over the streamable HTTP transport — the
same data that powers wellness-platform's in-app AI assistant,
now accessible to any MCP client.

## Why

The wellness-platform's tool-calling logic was built for its own
in-app chat assistant. This server exposes a slice of that same
data through the standard MCP protocol instead, so it can be used
by other MCP clients (Claude Desktop, the MCP Inspector, or a
custom agent) without going through the app's UI.

## Tools

- **`search_recipes`** — search by free text, cuisine, meal type,
  dietary tags, and max prep time.
- **`get_recipe_nutrition`** — full nutrition detail for a recipe
  by id: totals, per-serving figures, and a per-ingredient
  breakdown.

Both are read-only against the wellness-platform's existing
PostgreSQL database (shared instance, no schema changes, no writes).

## Configuration

Copy `.env.example` to `.env` (or just export the variable directly)
and fill in your own database connection string:

```bash
cp .env.example .env
```

| Variable       | Required | Description                                    |
|----------------|----------|-------------------------------------------------|
| `DATABASE_URL` | Yes      | PostgreSQL connection string (read-only usage)  |
| `PORT`         | No       | Defaults to `8080`; set automatically by Render |

The real `.env` file is git-ignored and never committed. In
production, `DATABASE_URL` is set directly in Render's environment
variable dashboard for the service, not in the repo.

## Run locally

```bash
export DATABASE_URL="postgres://user:pass@host:port/dbname?sslmode=require"
go run .
```

Server listens on `:8080` (or `$PORT`). MCP endpoint: `/mcp`.
Health check: `/healthz`.

## Testing

The [MCP Inspector](https://github.com/modelcontextprotocol/inspector)
is the fastest way to try it interactively:

```bash
npx @modelcontextprotocol/inspector
```

Set transport to **Streamable HTTP**, URL to `http://localhost:8080/mcp`,
connect, and call the tools from the **Tools** tab.

## Deployment

Deployed as a live service on Render using the streamable HTTP
transport, so it can be added as a custom connector by URL without
anyone needing to clone or build it locally.

## Architecture notes

- **Stateless streamable HTTP** — no session persistence needed,
  since every tool call is a self-contained read.
- **Shared database** — reuses the wellness-platform's existing
  Render Postgres instance rather than a separate one, avoiding
  reseeding when the free-tier database is periodically recreated.
- **Independent repo** — deliberately separate from the
  wellness-platform repo so each stands on its own.

## What's next

A multi-agent layer (Planner/Reviewer agents with tool use and a
feedback loop) is planned as a separate project on top of this
server's tools, plus observability via Langfuse. Not part of this
repo.