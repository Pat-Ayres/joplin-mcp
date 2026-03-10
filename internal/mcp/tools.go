package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Pat-Ayres/joplin-mcp/internal/joplin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTools adds all Joplin MCP tools to the given server.
func RegisterTools(s *server.MCPServer, client joplin.JoplinClient) {
	s.AddTool(listNotebooksTool(), listNotebooksHandler(client))
	s.AddTool(listNotesTool(), listNotesHandler(client))
	s.AddTool(getNoteTool(), getNoteHandler(client))
	s.AddTool(createNoteTool(), createNoteHandler(client))
	s.AddTool(appendToNoteTool(), appendToNoteHandler(client))
	s.AddTool(searchNotesTool(), searchNotesHandler(client))
	s.AddTool(createNotebookTool(), createNotebookHandler(client))
	s.AddTool(createTodoTool(), createTodoHandler(client))
	s.AddTool(markTodoCompletedTool(), markTodoCompletedHandler(client))
	s.AddTool(markTodoUncompletedTool(), markTodoUncompletedHandler(client))
	s.AddTool(listTodosTool(), listTodosHandler(client))
}

// --- list_notebooks ---

func listNotebooksTool() mcp.Tool {
	return mcp.NewTool("list_notebooks",
		mcp.WithDescription("List all Joplin notebooks (folders) with their names and IDs"),
	)
}

func listNotebooksHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		notebooks, err := client.ListNotebooks()
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to list notebooks: %v", err)), nil
		}

		data, err := json.MarshalIndent(notebooks, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to format notebooks: %v", err)), nil
		}

		return textResult(string(data)), nil
	}
}

// --- list_notes ---

func listNotesTool() mcp.Tool {
	return mcp.NewTool("list_notes",
		mcp.WithDescription("List all notes in a given Joplin notebook"),
		mcp.WithString("notebook_id",
			mcp.Description("The ID of the notebook to list notes from"),
			mcp.Required(),
		),
	)
}

func listNotesHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		notebookID := request.GetString("notebook_id", "")
		if notebookID == "" {
			return errorResult("notebook_id is required"), nil
		}

		notes, err := client.ListNotes(notebookID)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to list notes: %v", err)), nil
		}

		data, err := json.MarshalIndent(notes, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to format notes: %v", err)), nil
		}

		return textResult(string(data)), nil
	}
}

// --- get_note ---

func getNoteTool() mcp.Tool {
	return mcp.NewTool("get_note",
		mcp.WithDescription("Fetch the full content of a Joplin note by its ID"),
		mcp.WithString("note_id",
			mcp.Description("The ID of the note to fetch"),
			mcp.Required(),
		),
	)
}

func getNoteHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		noteID := request.GetString("note_id", "")
		if noteID == "" {
			return errorResult("note_id is required"), nil
		}

		note, err := client.GetNote(noteID)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to get note: %v", err)), nil
		}

		data, err := json.MarshalIndent(note, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to format note: %v", err)), nil
		}

		return textResult(string(data)), nil
	}
}

// --- create_note ---

func createNoteTool() mcp.Tool {
	return mcp.NewTool("create_note",
		mcp.WithDescription("Create a new note in a specified Joplin notebook"),
		mcp.WithString("title",
			mcp.Description("The title of the new note"),
			mcp.Required(),
		),
		mcp.WithString("body",
			mcp.Description("The markdown body content of the note"),
			mcp.Required(),
		),
		mcp.WithString("notebook_id",
			mcp.Description("The ID of the notebook to create the note in"),
			mcp.Required(),
		),
	)
}

func createNoteHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title := request.GetString("title", "")
		body := request.GetString("body", "")
		notebookID := request.GetString("notebook_id", "")

		if title == "" {
			return errorResult("title is required"), nil
		}
		if notebookID == "" {
			return errorResult("notebook_id is required"), nil
		}

		note, err := client.CreateNote(title, body, notebookID)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to create note: %v", err)), nil
		}

		data, err := json.MarshalIndent(note, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to format note: %v", err)), nil
		}

		return textResult(fmt.Sprintf("Note created successfully:\n%s", string(data))), nil
	}
}

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

func appendToNoteHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
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

func searchNotesTool() mcp.Tool {
	return mcp.NewTool("search_notes",
		mcp.WithDescription("Full-text search across all Joplin notes"),
		mcp.WithString("query",
			mcp.Description("The search query string"),
			mcp.Required(),
		),
	)
}

func searchNotesHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := request.GetString("query", "")
		if query == "" {
			return errorResult("query is required"), nil
		}

		notes, err := client.SearchNotes(query)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to search notes: %v", err)), nil
		}

		if len(notes) == 0 {
			return textResult("No notes found matching the query."), nil
		}

		data, err := json.MarshalIndent(notes, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to format results: %v", err)), nil
		}

		return textResult(fmt.Sprintf("Found %d note(s):\n%s", len(notes), string(data))), nil
	}
}

// --- create_notebook ---

func createNotebookTool() mcp.Tool {
	return mcp.NewTool("create_notebook",
		mcp.WithDescription("Create a new Joplin notebook (folder)"),
		mcp.WithString("title",
			mcp.Description("The title of the new notebook"),
			mcp.Required(),
		),
		mcp.WithString("parent_id",
			mcp.Description("Optional parent notebook ID for creating a sub-notebook"),
		),
	)
}

func createNotebookHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title := request.GetString("title", "")
		parentID := request.GetString("parent_id", "")

		if title == "" {
			return errorResult("title is required"), nil
		}

		notebook, err := client.CreateNotebook(title, parentID)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to create notebook: %v", err)), nil
		}

		data, err := json.MarshalIndent(notebook, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to format notebook: %v", err)), nil
		}

		return textResult(fmt.Sprintf("Notebook created successfully:\n%s", string(data))), nil
	}
}

// --- create_todo ---

func createTodoTool() mcp.Tool {
	return mcp.NewTool("create_todo",
		mcp.WithDescription("Create a new to-do note in a specified Joplin notebook"),
		mcp.WithString("title",
			mcp.Description("The title of the to-do"),
			mcp.Required(),
		),
		mcp.WithString("body",
			mcp.Description("The markdown body content of the to-do"),
			mcp.Required(),
		),
		mcp.WithString("notebook_id",
			mcp.Description("The ID of the notebook to create the to-do in"),
			mcp.Required(),
		),
		mcp.WithString("due_date",
			mcp.Description("Optional due date in YYYY-MM-DD format (e.g. 2025-12-31), or Unix timestamp in milliseconds"),
		),
	)
}

func createTodoHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title := request.GetString("title", "")
		body := request.GetString("body", "")
		notebookID := request.GetString("notebook_id", "")
		dueDateStr := request.GetString("due_date", "")

		if title == "" {
			return errorResult("title is required"), nil
		}
		if notebookID == "" {
			return errorResult("notebook_id is required"), nil
		}

		var dueUnixMs int64
		if dueDateStr != "" {
			parsed, err := parseDueDate(dueDateStr)
			if err != nil {
				return errorResult(fmt.Sprintf("invalid due_date: %v", err)), nil
			}
			dueUnixMs = parsed
		}

		note, err := client.CreateTodo(title, body, notebookID, dueUnixMs)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to create todo: %v", err)), nil
		}

		data, err := json.MarshalIndent(note, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to format note: %v", err)), nil
		}

		return textResult(fmt.Sprintf("Todo created successfully:\n%s", string(data))), nil
	}
}

// parseDueDate parses due_date from either YYYY-MM-DD or a Unix ms string.
func parseDueDate(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	// Try Unix milliseconds first (numeric string)
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return ms, nil
	}
	// Then try YYYY-MM-DD (start of day UTC)
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, fmt.Errorf("use YYYY-MM-DD or Unix ms: %w", err)
	}
	return t.UTC().UnixMilli(), nil
}

// --- mark_todo_completed ---

func markTodoCompletedTool() mcp.Tool {
	return mcp.NewTool("mark_todo_completed",
		mcp.WithDescription("Mark a to-do note as completed"),
		mcp.WithString("note_id",
			mcp.Description("The ID of the to-do note to mark as completed"),
			mcp.Required(),
		),
	)
}

func markTodoCompletedHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		noteID := request.GetString("note_id", "")
		if noteID == "" {
			return errorResult("note_id is required"), nil
		}

		note, err := client.MarkTodoCompleted(noteID)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to mark todo completed: %v", err)), nil
		}

		return textResult(fmt.Sprintf("Todo %q (ID: %s) marked as completed.", note.Title, note.ID)), nil
	}
}

// --- mark_todo_uncompleted ---

func markTodoUncompletedTool() mcp.Tool {
	return mcp.NewTool("mark_todo_uncompleted",
		mcp.WithDescription("Mark a to-do note as not completed"),
		mcp.WithString("note_id",
			mcp.Description("The ID of the to-do note to mark as uncompleted"),
			mcp.Required(),
		),
	)
}

func markTodoUncompletedHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		noteID := request.GetString("note_id", "")
		if noteID == "" {
			return errorResult("note_id is required"), nil
		}

		note, err := client.MarkTodoUncompleted(noteID)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to mark todo uncompleted: %v", err)), nil
		}

		return textResult(fmt.Sprintf("Todo %q (ID: %s) marked as uncompleted.", note.Title, note.ID)), nil
	}
}

// --- list_todos ---

func listTodosTool() mcp.Tool {
	return mcp.NewTool("list_todos",
		mcp.WithDescription("List to-do notes, optionally scoped to a notebook. Omit notebook_id to list all todos."),
		mcp.WithString("notebook_id",
			mcp.Description("Optional notebook ID to list only todos in that notebook"),
		),
	)
}

func listTodosHandler(client joplin.JoplinClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		notebookID := request.GetString("notebook_id", "")

		todos, err := client.ListTodos(notebookID)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to list todos: %v", err)), nil
		}

		if len(todos) == 0 {
			return textResult("No todos found."), nil
		}

		data, err := json.MarshalIndent(todos, "", "  ")
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to format todos: %v", err)), nil
		}

		return textResult(fmt.Sprintf("Found %d todo(s):\n%s", len(todos), string(data))), nil
	}
}

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
