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
