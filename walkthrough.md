# joplin-mcp: Building a Joplin MCP Server in Go

*2026-03-01T06:40:33Z by Showboat 0.6.1*
<!-- showboat-id: 9ccc32cd-b07e-46e5-b6eb-0f4d172a54b5 -->

This walkthrough documents the implementation of **joplin-mcp**, a Go-based MCP (Model Context Protocol) server that wraps Joplin's local REST API. It enables AI assistants like Claude Code to read, search, create, and append to Joplin notes directly through MCP tools.

## Project Structure

The repository follows standard Go project layout with three main packages:

```bash
find . -type f \( -name '*.go' -o -name '*.yaml' -o -name 'Dockerfile' -o -name '*.md' \) | grep -v vendor | grep -v '.git' | sort
```

```output
./CLAUDE.md
./Dockerfile
./README.md
./cmd/server/main.go
./config.yaml
./internal/joplin/client.go
./internal/mcp/tools.go
./walkthrough.md
```

## The Joplin REST API Client

The core of the server is a Go HTTP client that talks to Joplin's local REST API. Joplin's Web Clipper service exposes a REST API at `http://localhost:41184` with token-based authentication via query parameter.

Let's look at the client struct and its constructor:

```bash
sed -n '1,36p' internal/joplin/client.go
```

```output
package joplin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client wraps the Joplin REST API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new Joplin API client.
func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:41184"
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTPClient: &http.Client{},
	}
}

// Notebook represents a Joplin notebook (folder).
type Notebook struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	ParentID string `json:"parent_id"`
}
```

The client authenticates every request by appending `?token=...` as a query parameter. Here's the core HTTP helper that handles this:

```bash
sed -n '/^\/\/ doRequest/,/^}/p' internal/joplin/client.go
```

```output
// doRequest executes an HTTP request against the Joplin API.
func (c *Client) doRequest(method, path string, body io.Reader) ([]byte, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	q := u.Query()
	q.Set("token", c.Token)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Joplin API error (HTTP %d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}
```

Joplin's API paginates list responses. The client handles this transparently with `fetchAllPages`, which collects all items across pages before returning:

```bash
sed -n '/^\/\/ fetchAllPages/,/^}/p' internal/joplin/client.go
```

```output
// fetchAllPages collects all items from a paginated Joplin endpoint.
func (c *Client) fetchAllPages(path string, extraParams url.Values) ([]json.RawMessage, error) {
	var allItems []json.RawMessage
	page := 1

	for {
		u, err := url.Parse(c.BaseURL + path)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		q := u.Query()
		q.Set("token", c.Token)
		q.Set("page", fmt.Sprintf("%d", page))
		for k, vs := range extraParams {
			for _, v := range vs {
				q.Set(k, v)
			}
		}
		u.RawQuery = q.Encode()

		resp, err := c.HTTPClient.Get(u.String())
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Joplin API error (HTTP %d): %s", resp.StatusCode, string(data))
		}

		var pr paginatedResponse
		if err := json.Unmarshal(data, &pr); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		var items []json.RawMessage
		if err := json.Unmarshal(pr.Items, &items); err != nil {
			return nil, fmt.Errorf("decoding items: %w", err)
		}
		allItems = append(allItems, items...)

		if !pr.HasMore {
			break
		}
		page++
	}

	return allItems, nil
}
```

A key feature for AI workflows is `AppendToNote`, which fetches the existing note body and concatenates new content. This is the most useful operation for logging and journaling workflows:

```bash
sed -n '/^\/\/ AppendToNote/,/^}/p' internal/joplin/client.go
```

```output
// AppendToNote appends content to an existing note's body.
func (c *Client) AppendToNote(noteID, content string) (*Note, error) {
	existing, err := c.GetNote(noteID)
	if err != nil {
		return nil, fmt.Errorf("fetching note to append: %w", err)
	}

	newBody := existing.Body + "\n" + content
	payload := map[string]string{
		"body": newBody,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding note: %w", err)
	}

	data, err := c.doRequest("PUT", fmt.Sprintf("/notes/%s", noteID), strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("updating note: %w", err)
	}

	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		return nil, fmt.Errorf("decoding note: %w", err)
	}
	return &note, nil
}
```

## MCP Tool Definitions

Tools are defined using the `mcp-go` SDK's builder pattern. Each tool gets a definition (name, description, parameter schema) and a handler function. All 7 tools are registered in a single `RegisterTools` function:

```bash
sed -n '1,17p' internal/mcp/tools.go
```

```output
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Pat-Ayres/joplin-mcp/internal/joplin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTools adds all Joplin MCP tools to the given server.
func RegisterTools(s *server.MCPServer, client *joplin.Client) {
	s.AddTool(listNotebooksTool(), listNotebooksHandler(client))
	s.AddTool(listNotesTool(), listNotesHandler(client))
	s.AddTool(getNoteTool(), getNoteHandler(client))
```

Here's an example of how a tool definition and its handler pair work together. The `append_to_note` tool demonstrates the pattern used throughout — a tool definition function returns the schema, and a handler factory function closes over the Joplin client:

```bash
sed -n '/^\/\/ --- append_to_note/,/^\/\/ --- search_notes/p' internal/mcp/tools.go | head -40
```

```output
// --- append_to_note ---

func appendToNoteTool() mcp.Tool {
	return mcp.NewTool("append_to_note",
		mcp.WithDescription("Append content to an existing Joplin note"),
		mcp.WithString("note_id",
			mcp.Description("The ID of the note to append to"),
			mcp.Required(),
		),
		mcp.WithString("content",
			mcp.Description("The markdown content to append to the note"),
			mcp.Required(),
		),
	)
}

func appendToNoteHandler(client *joplin.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		noteID := request.GetString("note_id", "")
		content := request.GetString("content", "")

		if noteID == "" {
			return errorResult("note_id is required"), nil
		}
		if content == "" {
			return errorResult("content is required"), nil
		}

		note, err := client.AppendToNote(noteID, content)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to append to note: %v", err)), nil
		}

		return textResult(fmt.Sprintf("Content appended successfully to note %q (ID: %s)", note.Title, note.ID)), nil
	}
}

// --- search_notes ---
```

Notice the error handling convention: tool-level errors return an `errorResult()` (which sets `IsError: true` on the MCP response) rather than a Go error. This ensures Claude sees a human-readable message instead of a transport failure. The helper functions at the bottom of the file keep this pattern clean:

```bash
sed -n '/^\/\/ --- helpers/,$p' internal/mcp/tools.go
```

```output
// --- helpers ---

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: text,
			},
		},
	}
}

func errorResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: message,
			},
		},
	}
}
```

## Server Entrypoint

The `main.go` wires everything together: load config from environment variables, create the Joplin client, set up the MCP server with tool capabilities, register all tools, and start serving over stdio:

```bash
cat cmd/server/main.go
```

```output
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Pat-Ayres/joplin-mcp/internal/joplin"
	mcptools "github.com/Pat-Ayres/joplin-mcp/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	token := os.Getenv("JOPLIN_API_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "JOPLIN_API_TOKEN environment variable is required")
		os.Exit(1)
	}

	baseURL := os.Getenv("JOPLIN_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:41184"
	}

	client := joplin.NewClient(baseURL, token)

	s := server.NewMCPServer(
		"joplin-mcp",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	mcptools.RegisterTools(s, client)

	log.Println("Starting joplin-mcp server (stdio)...")
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
```

The import alias `mcptools` avoids a name collision between our `internal/mcp` package and the SDK's `mcp` package. The server uses `server.WithToolCapabilities(true)` to advertise dynamic tool list changes (though our tool set is static, it's a safe default).

`server.ServeStdio` handles the stdio transport, signal handling (SIGTERM/SIGINT), and JSON-RPC message routing automatically.

## Docker Support

The Dockerfile uses a multi-stage build — a Go builder stage compiles the binary, then copies it into a minimal Alpine image:

```bash
cat Dockerfile
```

```output
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o joplin-mcp ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /app/joplin-mcp /usr/local/bin/joplin-mcp

ENTRYPOINT ["joplin-mcp"]
```

## Build Verification

Let's confirm the project compiles and passes static analysis:

```bash
go build -v ./cmd/server/ 2>&1 && echo 'Build succeeded'
```

```output
Build succeeded
```

```bash
go vet ./... 2>&1 && echo 'Vet passed: no issues found'
```

```output
Vet passed: no issues found
```

## Summary of MCP Tools

Here's a quick reference of all 7 tools exposed by the server:

```bash
grep 'mcp.NewTool(' internal/mcp/tools.go | grep -v '//' | sed 's/.*mcp.NewTool("//' | sed 's/",$//' | sort
```

```output
append_to_note
create_note
create_notebook
get_note
list_notebooks
list_notes
search_notes
```

Each tool maps directly to a Joplin REST API operation:
- **list_notebooks** / **list_notes** — Read-only listing with automatic pagination
- **get_note** — Fetches full note content (title + markdown body)
- **create_note** / **create_notebook** — Write operations for new content
- **append_to_note** — The workhorse for AI workflows: appends markdown to existing notes
- **search_notes** — Full-text search leveraging Joplin's built-in search engine

The server is ready to use with Claude Code via the MCP server configuration shown in the README.
