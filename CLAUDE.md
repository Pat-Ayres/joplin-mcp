# joplin-mcp

Go MCP server wrapping the Joplin REST API.

## Build & Test

```bash
go build ./cmd/server/       # build the server binary
go vet ./...                  # static analysis
go test ./...                 # run tests (when added)
```

## Architecture

- `cmd/server/main.go` — Entrypoint. Loads config from env vars, creates Joplin client, registers MCP tools, starts stdio server.
- `internal/joplin/client.go` — HTTP client for Joplin REST API. Handles pagination, auth token as query param.
- `internal/mcp/tools.go` — MCP tool definitions using `github.com/mark3labs/mcp-go`. Each tool maps to a Joplin client method.

## Conventions

- MCP tools return user-facing errors via `errorResult()` (sets `IsError: true`), not Go errors. Go errors are reserved for transport-level failures.
- Joplin API token is passed as `?token=...` query parameter on every request.
- Server runs in stdio mode only (no HTTP transport).
- All tool handlers live in `internal/mcp/tools.go` and follow the pattern: tool definition function + handler factory function.

## Environment Variables

- `JOPLIN_API_TOKEN` (required) — Joplin Web Clipper API token
- `JOPLIN_API_URL` (optional) — Joplin API base URL, defaults to `http://localhost:41184`
