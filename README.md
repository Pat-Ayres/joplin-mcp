# joplin-mcp

A Go-based [MCP](https://modelcontextprotocol.io/) server that wraps the [Joplin](https://joplinapp.org/) local REST API, enabling Claude Code (and other MCP clients) to read and write Joplin notes directly.

## Features

| Tool | Description |
|------|-------------|
| `list_notebooks` | List all notebook names and IDs |
| `list_notes` | List notes within a given notebook |
| `get_note` | Fetch a note's full content by ID |
| `create_note` | Create a new note in a specified notebook |
| `append_to_note` | Append content to an existing note |
| `search_notes` | Full-text search across all notes |
| `create_notebook` | Create a new notebook |

## Prerequisites

- Go 1.23+
- Joplin desktop app with the Web Clipper service enabled (Tools > Options > Web Clipper)
- Joplin API token (found in the Web Clipper settings)

## Build

```bash
go build -o joplin-mcp ./cmd/server
```

## Configuration

The server requires a Joplin API token. Set it via environment variable:

```bash
export JOPLIN_API_TOKEN="your_token_here"
```

Optionally set a custom API URL (defaults to `http://localhost:41184`):

```bash
export JOPLIN_API_URL="http://localhost:41184"
```

## Usage with Claude Code

Add the server to your Claude Code MCP configuration (`~/.claude/claude_code_config.json`):

```json
{
  "mcpServers": {
    "joplin": {
      "command": "/path/to/joplin-mcp",
      "env": {
        "JOPLIN_API_TOKEN": "your_token_here"
      }
    }
  }
}
```

Then Claude Code can use tools like `list_notebooks`, `search_notes`, `append_to_note`, etc.

## Docker

```bash
docker build -t joplin-mcp .
docker run --rm -e JOPLIN_API_TOKEN="your_token" joplin-mcp
```

Note: When running in Docker, the container needs network access to the Joplin API. Use `--network host` or configure the `JOPLIN_API_URL` to point to the host.

## Architecture

```
cmd/server/main.go          Entrypoint: config loading, wiring, stdio server
internal/joplin/client.go   Joplin REST API client (HTTP, pagination)
internal/mcp/tools.go       MCP tool definitions and handlers
```

The server runs in **stdio mode**, which is the standard transport for Claude Code MCP integrations. It reads JSON-RPC messages from stdin and writes responses to stdout.

## License

See [LICENSE](LICENSE).
