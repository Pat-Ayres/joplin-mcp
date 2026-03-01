# joplin-mcp

Go MCP server wrapping the Joplin REST API.

## Build & Test

```bash
go build ./cmd/server/       # build the server binary
go vet ./...                  # static analysis
go test ./internal/...        # unit + integration tests
make test                     # vet + unit tests
make test-e2e                 # e2e tests via Docker Compose (requires Docker)
```

## Architecture

- `cmd/server/main.go` — Entrypoint. Loads config from env vars, creates Joplin client, registers MCP tools, starts stdio server.
- `internal/joplin/client.go` — HTTP client for Joplin REST API. Handles pagination, auth token as query param. Exports `JoplinClient` interface.
- `internal/mcp/tools.go` — MCP tool definitions using `github.com/mark3labs/mcp-go`. Each tool maps to a `JoplinClient` method.
- `e2e/` — E2E test suite (build tag `e2e`) + custom headless Joplin Docker image.

## Conventions

- MCP tools return user-facing errors via `errorResult()` (sets `IsError: true`), not Go errors. Go errors are reserved for transport-level failures.
- Joplin API token is passed as `?token=...` query parameter on every request.
- Server runs in stdio mode only (no HTTP transport).
- All tool handlers live in `internal/mcp/tools.go` and follow the pattern: tool definition function + handler factory function.
- Tool handler tests use `mcptest` package from the mcp-go SDK for protocol-level validation.
- E2E tests are behind a `//go:build e2e` build tag and require `JOPLIN_API_URL` + `JOPLIN_API_TOKEN` env vars.

## Environment Variables

- `JOPLIN_API_TOKEN` (required) — Joplin Web Clipper API token
- `JOPLIN_API_URL` (optional) — Joplin API base URL, defaults to `http://localhost:41184`
