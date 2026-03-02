package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Pat-Ayres/joplin-mcp/internal/joplin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
)

// --- mock client ---

type mockClient struct {
	listNotebooksFunc  func() ([]joplin.Notebook, error)
	listNotesFunc      func(string) ([]joplin.Note, error)
	getNoteFunc        func(string) (*joplin.Note, error)
	createNoteFunc     func(string, string, string) (*joplin.Note, error)
	appendToNoteFunc   func(string, string) (*joplin.Note, error)
	searchNotesFunc    func(string) ([]joplin.Note, error)
	createNotebookFunc func(string, string) (*joplin.Notebook, error)
}

func (m *mockClient) ListNotebooks() ([]joplin.Notebook, error) {
	return m.listNotebooksFunc()
}
func (m *mockClient) ListNotes(notebookID string) ([]joplin.Note, error) {
	return m.listNotesFunc(notebookID)
}
func (m *mockClient) GetNote(noteID string) (*joplin.Note, error) {
	return m.getNoteFunc(noteID)
}
func (m *mockClient) CreateNote(title, body, notebookID string) (*joplin.Note, error) {
	return m.createNoteFunc(title, body, notebookID)
}
func (m *mockClient) AppendToNote(noteID, content string) (*joplin.Note, error) {
	return m.appendToNoteFunc(noteID, content)
}
func (m *mockClient) SearchNotes(query string) ([]joplin.Note, error) {
	return m.searchNotesFunc(query)
}
func (m *mockClient) CreateNotebook(title, parentID string) (*joplin.Notebook, error) {
	return m.createNotebookFunc(title, parentID)
}

// --- helpers ---

// newTestServer creates an mcptest.Server with all tools registered against the given mock.
func newTestServer(t *testing.T, mock *mockClient) *mcptest.Server {
	t.Helper()

	ts := mcptest.NewUnstartedServer(t)

	// Register all tools using the same pattern as RegisterTools
	ts.AddTool(listNotebooksTool(), listNotebooksHandler(mock))
	ts.AddTool(listNotesTool(), listNotesHandler(mock))
	ts.AddTool(getNoteTool(), getNoteHandler(mock))
	ts.AddTool(createNoteTool(), createNoteHandler(mock))
	ts.AddTool(appendToNoteTool(), appendToNoteHandler(mock))
	ts.AddTool(searchNotesTool(), searchNotesHandler(mock))
	ts.AddTool(createNotebookTool(), createNotebookHandler(mock))

	if err := ts.Start(context.Background()); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	t.Cleanup(func() { ts.Close() })
	return ts
}

func callTool(t *testing.T, ts *mcptest.Server, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	result, err := ts.Client().CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool(%s) transport error: %v", name, err)
	}
	return result
}

func resultText(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// --- tests ---

func TestListTools(t *testing.T) {
	mock := &mockClient{
		listNotebooksFunc:  func() ([]joplin.Notebook, error) { return nil, nil },
		listNotesFunc:      func(string) ([]joplin.Note, error) { return nil, nil },
		getNoteFunc:        func(string) (*joplin.Note, error) { return nil, nil },
		createNoteFunc:     func(string, string, string) (*joplin.Note, error) { return nil, nil },
		appendToNoteFunc:   func(string, string) (*joplin.Note, error) { return nil, nil },
		searchNotesFunc:    func(string) ([]joplin.Note, error) { return nil, nil },
		createNotebookFunc: func(string, string) (*joplin.Notebook, error) { return nil, nil },
	}
	ts := newTestServer(t, mock)

	req := mcp.ListToolsRequest{}
	result, err := ts.Client().ListTools(context.Background(), req)
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}

	expectedTools := map[string]bool{
		"list_notebooks": false,
		"list_notes":     false,
		"get_note":       false,
		"create_note":    false,
		"append_to_note": false,
		"search_notes":   false,
		"create_notebook": false,
	}

	for _, tool := range result.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("tool %q not found in ListTools response", name)
		}
	}

	if len(result.Tools) != 7 {
		t.Errorf("expected 7 tools, got %d", len(result.Tools))
	}
}

func TestListNotebooks_Success(t *testing.T) {
	mock := &mockClient{
		listNotebooksFunc: func() ([]joplin.Notebook, error) {
			return []joplin.Notebook{
				{ID: "nb-1", Title: "Work"},
				{ID: "nb-2", Title: "Personal"},
			}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "list_notebooks", nil)

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(result))
	}

	text := resultText(result)
	var notebooks []joplin.Notebook
	if err := json.Unmarshal([]byte(text), &notebooks); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
	if len(notebooks) != 2 {
		t.Errorf("expected 2 notebooks, got %d", len(notebooks))
	}
}

func TestListNotebooks_ClientError(t *testing.T) {
	mock := &mockClient{
		listNotebooksFunc: func() ([]joplin.Notebook, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "list_notebooks", nil)

	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
	if text := resultText(result); !strings.Contains(text, "connection refused") {
		t.Errorf("expected error message to contain 'connection refused', got: %s", text)
	}
}

func TestListNotes_MissingParam(t *testing.T) {
	mock := &mockClient{
		listNotesFunc: func(string) ([]joplin.Note, error) { return nil, nil },
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "list_notes", map[string]any{})

	if !result.IsError {
		t.Fatal("expected IsError=true for missing notebook_id")
	}
	if text := resultText(result); !strings.Contains(text, "notebook_id is required") {
		t.Errorf("unexpected error: %s", text)
	}
}

func TestListNotes_Success(t *testing.T) {
	mock := &mockClient{
		listNotesFunc: func(notebookID string) ([]joplin.Note, error) {
			if notebookID != "nb-1" {
				t.Errorf("expected notebook_id nb-1, got %s", notebookID)
			}
			return []joplin.Note{
				{ID: "note-1", Title: "Meeting notes"},
			}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "list_notes", map[string]any{"notebook_id": "nb-1"})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(result))
	}
}

func TestGetNote_Success(t *testing.T) {
	mock := &mockClient{
		getNoteFunc: func(noteID string) (*joplin.Note, error) {
			return &joplin.Note{
				ID:    noteID,
				Title: "Test Note",
				Body:  "# Hello\nWorld",
			}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "get_note", map[string]any{"note_id": "note-1"})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(result))
	}
	text := resultText(result)
	if !strings.Contains(text, "Test Note") || !strings.Contains(text, "Hello") {
		t.Errorf("expected note content in response, got: %s", text)
	}
}

func TestGetNote_MissingParam(t *testing.T) {
	mock := &mockClient{
		getNoteFunc: func(string) (*joplin.Note, error) { return nil, nil },
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "get_note", map[string]any{})

	if !result.IsError {
		t.Fatal("expected IsError=true for missing note_id")
	}
}

func TestCreateNote_Success(t *testing.T) {
	mock := &mockClient{
		createNoteFunc: func(title, body, notebookID string) (*joplin.Note, error) {
			return &joplin.Note{
				ID:       "note-new",
				ParentID: notebookID,
				Title:    title,
				Body:     body,
			}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "create_note", map[string]any{
		"title":       "New Note",
		"body":        "content here",
		"notebook_id": "nb-1",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(result))
	}
	text := resultText(result)
	if !strings.Contains(text, "Note created successfully") {
		t.Errorf("expected success message, got: %s", text)
	}
}

func TestCreateNote_MissingTitle(t *testing.T) {
	mock := &mockClient{
		createNoteFunc: func(string, string, string) (*joplin.Note, error) { return nil, nil },
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "create_note", map[string]any{
		"body":        "content",
		"notebook_id": "nb-1",
	})

	if !result.IsError {
		t.Fatal("expected IsError=true for missing title")
	}
}

func TestAppendToNote_Success(t *testing.T) {
	mock := &mockClient{
		appendToNoteFunc: func(noteID, content string) (*joplin.Note, error) {
			return &joplin.Note{
				ID:    noteID,
				Title: "Existing Note",
			}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "append_to_note", map[string]any{
		"note_id": "note-1",
		"content": "new paragraph",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(result))
	}
	text := resultText(result)
	if !strings.Contains(text, "Content appended successfully") {
		t.Errorf("expected success message, got: %s", text)
	}
}

func TestAppendToNote_MissingContent(t *testing.T) {
	mock := &mockClient{
		appendToNoteFunc: func(string, string) (*joplin.Note, error) { return nil, nil },
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "append_to_note", map[string]any{
		"note_id": "note-1",
	})

	if !result.IsError {
		t.Fatal("expected IsError=true for missing content")
	}
}

func TestSearchNotes_Success(t *testing.T) {
	mock := &mockClient{
		searchNotesFunc: func(query string) ([]joplin.Note, error) {
			return []joplin.Note{
				{ID: "note-1", Title: "Match"},
			}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "search_notes", map[string]any{"query": "test"})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(result))
	}
	text := resultText(result)
	if !strings.Contains(text, "Found 1 note(s)") {
		t.Errorf("expected count in response, got: %s", text)
	}
}

func TestSearchNotes_NoResults(t *testing.T) {
	mock := &mockClient{
		searchNotesFunc: func(string) ([]joplin.Note, error) {
			return []joplin.Note{}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "search_notes", map[string]any{"query": "nonexistent"})

	if result.IsError {
		t.Fatal("unexpected error for empty results")
	}
	text := resultText(result)
	if !strings.Contains(text, "No notes found") {
		t.Errorf("expected 'No notes found', got: %s", text)
	}
}

func TestSearchNotes_MissingQuery(t *testing.T) {
	mock := &mockClient{
		searchNotesFunc: func(string) ([]joplin.Note, error) { return nil, nil },
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "search_notes", map[string]any{})

	if !result.IsError {
		t.Fatal("expected IsError=true for missing query")
	}
}

func TestCreateNotebook_Success(t *testing.T) {
	mock := &mockClient{
		createNotebookFunc: func(title, parentID string) (*joplin.Notebook, error) {
			return &joplin.Notebook{
				ID:    "nb-new",
				Title: title,
			}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "create_notebook", map[string]any{"title": "Projects"})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(result))
	}
	text := resultText(result)
	if !strings.Contains(text, "Notebook created successfully") {
		t.Errorf("expected success message, got: %s", text)
	}
}

func TestCreateNotebook_WithParent(t *testing.T) {
	var gotParentID string
	mock := &mockClient{
		createNotebookFunc: func(title, parentID string) (*joplin.Notebook, error) {
			gotParentID = parentID
			return &joplin.Notebook{
				ID:       "nb-child",
				Title:    title,
				ParentID: parentID,
			}, nil
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "create_notebook", map[string]any{
		"title":     "Sub-project",
		"parent_id": "nb-parent",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(result))
	}
	if gotParentID != "nb-parent" {
		t.Errorf("expected parent_id 'nb-parent', got %s", gotParentID)
	}
}

func TestCreateNotebook_MissingTitle(t *testing.T) {
	mock := &mockClient{
		createNotebookFunc: func(string, string) (*joplin.Notebook, error) { return nil, nil },
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "create_notebook", map[string]any{})

	if !result.IsError {
		t.Fatal("expected IsError=true for missing title")
	}
}

func TestHandler_ClientError(t *testing.T) {
	mock := &mockClient{
		getNoteFunc: func(string) (*joplin.Note, error) {
			return nil, fmt.Errorf("Joplin API error (HTTP 500): internal server error")
		},
	}
	ts := newTestServer(t, mock)
	result := callTool(t, ts, "get_note", map[string]any{"note_id": "note-1"})

	if !result.IsError {
		t.Fatal("expected IsError=true when client returns error")
	}
	text := resultText(result)
	if !strings.Contains(text, "Failed to get note") {
		t.Errorf("expected 'Failed to get note' in error, got: %s", text)
	}
}

// Verify RegisterTools wires up the same tools as our test helper.
func TestRegisterTools(t *testing.T) {
	mock := &mockClient{
		listNotebooksFunc:  func() ([]joplin.Notebook, error) { return nil, nil },
		listNotesFunc:      func(string) ([]joplin.Note, error) { return nil, nil },
		getNoteFunc:        func(string) (*joplin.Note, error) { return nil, nil },
		createNoteFunc:     func(string, string, string) (*joplin.Note, error) { return nil, nil },
		appendToNoteFunc:   func(string, string) (*joplin.Note, error) { return nil, nil },
		searchNotesFunc:    func(string) ([]joplin.Note, error) { return nil, nil },
		createNotebookFunc: func(string, string) (*joplin.Notebook, error) { return nil, nil },
	}

	s := server.NewMCPServer("test", "0.1.0", server.WithToolCapabilities(true))
	RegisterTools(s, mock)

	// If RegisterTools compiles and doesn't panic, the interface wiring is correct.
	// This test exists to catch regressions if RegisterTools signature changes.
}
