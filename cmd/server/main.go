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
