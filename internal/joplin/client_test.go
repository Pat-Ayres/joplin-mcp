package joplin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient creates a Client pointing at the given httptest.Server.
func newTestClient(ts *httptest.Server) *Client {
	return &Client{
		BaseURL:    ts.URL,
		Token:      "test-token",
		HTTPClient: ts.Client(),
	}
}

func TestTokenAuth(t *testing.T) {
	var gotToken string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("token")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"items":    []any{},
			"has_more": false,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.ListNotebooks()

	if gotToken != "test-token" {
		t.Errorf("expected token %q, got %q", "test-token", gotToken)
	}
}

func TestHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.GetNote("note-1")
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if got := err.Error(); !contains(got, "HTTP 500") {
		t.Errorf("expected error to mention HTTP 500, got: %s", got)
	}
}

func TestListNotebooks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if got := r.URL.Path; got != "/folders" {
			t.Errorf("expected path /folders, got %s", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]string{
				{"id": "nb-1", "title": "Notebook 1", "parent_id": ""},
				{"id": "nb-2", "title": "Notebook 2", "parent_id": "nb-1"},
			},
			"has_more": false,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	notebooks, err := c.ListNotebooks()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notebooks) != 2 {
		t.Fatalf("expected 2 notebooks, got %d", len(notebooks))
	}
	if notebooks[0].ID != "nb-1" || notebooks[0].Title != "Notebook 1" {
		t.Errorf("unexpected first notebook: %+v", notebooks[0])
	}
	if notebooks[1].ParentID != "nb-1" {
		t.Errorf("expected parent_id nb-1, got %s", notebooks[1].ParentID)
	}
}

func TestListNotebooks_Pagination(t *testing.T) {
	page := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			json.NewEncoder(w).Encode(map[string]any{
				"items":    []map[string]string{{"id": "nb-1", "title": "Page 1"}},
				"has_more": true,
			})
		case 2:
			json.NewEncoder(w).Encode(map[string]any{
				"items":    []map[string]string{{"id": "nb-2", "title": "Page 2"}},
				"has_more": false,
			})
		default:
			t.Errorf("unexpected page %d", page)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	notebooks, err := c.ListNotebooks()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notebooks) != 2 {
		t.Fatalf("expected 2 notebooks across pages, got %d", len(notebooks))
	}
	if page != 2 {
		t.Errorf("expected 2 page requests, got %d", page)
	}
}

func TestListNotes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/folders/nb-1/notes" {
			t.Errorf("expected path /folders/nb-1/notes, got %s", got)
		}
		if got := r.URL.Query().Get("fields"); got != "id,parent_id,title" {
			t.Errorf("expected fields=id,parent_id,title, got %s", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]string{
				{"id": "note-1", "parent_id": "nb-1", "title": "Note 1"},
			},
			"has_more": false,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	notes, err := c.ListNotes("nb-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Title != "Note 1" {
		t.Errorf("expected title 'Note 1', got %s", notes[0].Title)
	}
}

func TestGetNote(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		// The path includes query params due to how doRequest builds the URL
		if got := r.URL.Path; got != "/notes/note-1" {
			t.Errorf("expected path /notes/note-1, got %s", got)
		}
		json.NewEncoder(w).Encode(Note{
			ID:       "note-1",
			ParentID: "nb-1",
			Title:    "Test Note",
			Body:     "# Hello\nWorld",
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	note, err := c.GetNote("note-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note.Title != "Test Note" {
		t.Errorf("expected title 'Test Note', got %s", note.Title)
	}
	if note.Body != "# Hello\nWorld" {
		t.Errorf("unexpected body: %s", note.Body)
	}
}

func TestCreateNote(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/notes" {
			t.Errorf("expected path /notes, got %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)

		if payload["title"] != "New Note" {
			t.Errorf("expected title 'New Note', got %s", payload["title"])
		}
		if payload["body"] != "content" {
			t.Errorf("expected body 'content', got %s", payload["body"])
		}
		if payload["parent_id"] != "nb-1" {
			t.Errorf("expected parent_id 'nb-1', got %s", payload["parent_id"])
		}

		json.NewEncoder(w).Encode(Note{
			ID:       "note-new",
			ParentID: "nb-1",
			Title:    "New Note",
			Body:     "content",
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	note, err := c.CreateNote("New Note", "content", "nb-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note.ID != "note-new" {
		t.Errorf("expected ID note-new, got %s", note.ID)
	}
}

func TestAppendToNote(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch {
		case r.Method == http.MethodGet && callCount == 1:
			// GetNote call
			json.NewEncoder(w).Encode(Note{
				ID:    "note-1",
				Title: "Existing",
				Body:  "original content",
			})
		case r.Method == http.MethodPut && callCount == 2:
			// Update call
			body, _ := io.ReadAll(r.Body)
			var payload map[string]string
			json.Unmarshal(body, &payload)

			expected := "original content\nappended text"
			if payload["body"] != expected {
				t.Errorf("expected body %q, got %q", expected, payload["body"])
			}

			json.NewEncoder(w).Encode(Note{
				ID:    "note-1",
				Title: "Existing",
				Body:  payload["body"],
			})
		default:
			t.Errorf("unexpected call %d: %s %s", callCount, r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	note, err := c.AppendToNote("note-1", "appended text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note.Body != "original content\nappended text" {
		t.Errorf("unexpected body: %s", note.Body)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (GET+PUT), got %d", callCount)
	}
}

func TestSearchNotes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "search term" {
			t.Errorf("expected query 'search term', got %s", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]string{
				{"id": "note-1", "parent_id": "nb-1", "title": "Found Note"},
			},
			"has_more": false,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	notes, err := c.SearchNotes("search term")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 result, got %d", len(notes))
	}
	if notes[0].Title != "Found Note" {
		t.Errorf("expected title 'Found Note', got %s", notes[0].Title)
	}
}

func TestCreateNotebook(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/folders" {
			t.Errorf("expected path /folders, got %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)

		if payload["title"] != "New Notebook" {
			t.Errorf("expected title 'New Notebook', got %s", payload["title"])
		}
		if _, hasParent := payload["parent_id"]; hasParent {
			t.Error("expected no parent_id for top-level notebook")
		}

		json.NewEncoder(w).Encode(Notebook{
			ID:    "nb-new",
			Title: "New Notebook",
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	nb, err := c.CreateNotebook("New Notebook", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nb.ID != "nb-new" {
		t.Errorf("expected ID nb-new, got %s", nb.ID)
	}
}

func TestCreateNotebook_WithParent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)

		if payload["parent_id"] != "nb-parent" {
			t.Errorf("expected parent_id 'nb-parent', got %s", payload["parent_id"])
		}

		json.NewEncoder(w).Encode(Notebook{
			ID:       "nb-child",
			Title:    "Child Notebook",
			ParentID: "nb-parent",
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	nb, err := c.CreateNotebook("Child Notebook", "nb-parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nb.ParentID != "nb-parent" {
		t.Errorf("expected parent_id nb-parent, got %s", nb.ParentID)
	}
}

func TestCreateTodo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/notes" {
			t.Errorf("expected POST /notes, got %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)
		if title, _ := payload["title"].(string); title != "My Todo" {
			t.Errorf("expected title 'My Todo', got %v", payload["title"])
		}
		if bodyVal, _ := payload["body"].(string); bodyVal != "Do something" {
			t.Errorf("expected body 'Do something', got %v", payload["body"])
		}
		if parent, _ := payload["parent_id"].(string); parent != "nb-1" {
			t.Errorf("expected parent_id 'nb-1', got %v", payload["parent_id"])
		}
		if isTodo, _ := payload["is_todo"].(float64); isTodo != 1 {
			t.Errorf("expected is_todo 1, got %v", payload["is_todo"])
		}
		if _, hasDue := payload["todo_due"]; hasDue {
			t.Error("expected no todo_due when dueUnixMs is 0")
		}
		json.NewEncoder(w).Encode(Note{
			ID:       "todo-1",
			ParentID: "nb-1",
			Title:    "My Todo",
			Body:     "Do something",
			IsTodo:   1,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	note, err := c.CreateTodo("My Todo", "Do something", "nb-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note.ID != "todo-1" || note.IsTodo != 1 {
		t.Errorf("unexpected note: %+v", note)
	}
}

func TestCreateTodo_WithDue(t *testing.T) {
	dueMs := int64(1735689600000) // 2025-01-01 00:00:00 UTC
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)
		if payload["is_todo"].(float64) != 1 {
			t.Error("expected is_todo 1")
		}
		if got, _ := payload["todo_due"].(float64); int64(got) != dueMs {
			t.Errorf("expected todo_due %d, got %v", dueMs, payload["todo_due"])
		}
		json.NewEncoder(w).Encode(Note{
			ID:       "todo-2",
			ParentID: "nb-1",
			Title:    "Due Todo",
			IsTodo:   1,
			TodoDue:  dueMs,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	note, err := c.CreateTodo("Due Todo", "body", "nb-1", dueMs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note.TodoDue != dueMs {
		t.Errorf("expected TodoDue %d, got %d", dueMs, note.TodoDue)
	}
}

func TestMarkTodoCompleted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/notes/todo-1" {
			t.Errorf("expected path /notes/todo-1, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)
		if completed, ok := payload["todo_completed"].(float64); !ok || completed <= 0 {
			t.Errorf("expected todo_completed to be positive timestamp, got %v", payload["todo_completed"])
		}
		json.NewEncoder(w).Encode(Note{
			ID:            "todo-1",
			Title:         "Done",
			IsTodo:        1,
			TodoCompleted: 1704067200000,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	note, err := c.MarkTodoCompleted("todo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note.ID != "todo-1" || note.TodoCompleted == 0 {
		t.Errorf("unexpected note: %+v", note)
	}
}

func TestMarkTodoUncompleted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/notes/todo-1" {
			t.Errorf("expected PUT /notes/todo-1, got %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)
		if completed, _ := payload["todo_completed"].(float64); completed != 0 {
			t.Errorf("expected todo_completed 0, got %v", payload["todo_completed"])
		}
		json.NewEncoder(w).Encode(Note{
			ID:            "todo-1",
			Title:         "Reopened",
			IsTodo:        1,
			TodoCompleted: 0,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	note, err := c.MarkTodoUncompleted("todo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if note.TodoCompleted != 0 {
		t.Errorf("expected TodoCompleted 0, got %d", note.TodoCompleted)
	}
}

func TestListTodos_InNotebook(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/folders/nb-1/notes" {
			t.Errorf("expected path /folders/nb-1/notes, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("fields"); got != noteFieldsWithTodo {
			t.Errorf("expected fields %q, got %q", noteFieldsWithTodo, got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": "todo-1", "parent_id": "nb-1", "title": "Todo A", "is_todo": 1, "todo_due": 0, "todo_completed": 0},
				{"id": "note-1", "parent_id": "nb-1", "title": "Regular Note", "is_todo": 0, "todo_due": 0, "todo_completed": 0},
				{"id": "todo-2", "parent_id": "nb-1", "title": "Todo B", "is_todo": 1, "todo_due": 0, "todo_completed": 1735689600000},
			},
			"has_more": false,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	todos, err := c.ListTodos("nb-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos (filtered), got %d", len(todos))
	}
	if todos[0].Title != "Todo A" || todos[1].Title != "Todo B" {
		t.Errorf("unexpected todos: %+v", todos)
	}
}

func TestListTodos_All(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notes" {
			t.Errorf("expected path /notes, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": "todo-1", "parent_id": "nb-1", "title": "Only Todo", "is_todo": 1, "todo_due": 0, "todo_completed": 0},
			},
			"has_more": false,
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	todos, err := c.ListTodos("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(todos) != 1 || todos[0].ID != "todo-1" {
		t.Fatalf("expected 1 todo, got %d: %+v", len(todos), todos)
	}
}

// contains is a simple substring check helper.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
