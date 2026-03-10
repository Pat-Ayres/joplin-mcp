//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Pat-Ayres/joplin-mcp/internal/joplin"
)

var client *joplin.Client

func TestMain(m *testing.M) {
	apiURL := os.Getenv("JOPLIN_API_URL")
	token := os.Getenv("JOPLIN_API_TOKEN")

	if apiURL == "" || token == "" {
		fmt.Fprintln(os.Stderr, "JOPLIN_API_URL and JOPLIN_API_TOKEN must be set for e2e tests")
		os.Exit(1)
	}

	client = joplin.NewClient(apiURL, token)
	os.Exit(m.Run())
}

// TestE2EWorkflow runs subtests in order, each building on the previous one's state.
func TestE2EWorkflow(t *testing.T) {
	var notebookID string
	var noteID string
	var todoID string
	const (
		notebookTitle = "e2e-test-notebook"
		noteTitle     = "e2e-test-note"
		noteBody      = "This is the original body."
		appendContent = "This was appended."
		todoTitle     = "e2e-test-todo"
		todoBody      = "E2E todo body."
	)

	t.Run("CreateNotebook", func(t *testing.T) {
		nb, err := client.CreateNotebook(notebookTitle, "")
		if err != nil {
			t.Fatalf("CreateNotebook failed: %v", err)
		}
		if nb.ID == "" {
			t.Fatal("expected non-empty notebook ID")
		}
		if nb.Title != notebookTitle {
			t.Errorf("expected title %q, got %q", notebookTitle, nb.Title)
		}
		notebookID = nb.ID
		t.Logf("Created notebook: %s (ID: %s)", nb.Title, nb.ID)
	})

	t.Run("ListNotebooks", func(t *testing.T) {
		if notebookID == "" {
			t.Skip("depends on CreateNotebook")
		}
		notebooks, err := client.ListNotebooks()
		if err != nil {
			t.Fatalf("ListNotebooks failed: %v", err)
		}
		found := false
		for _, nb := range notebooks {
			if nb.ID == notebookID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("notebook %s not found in list of %d notebooks", notebookID, len(notebooks))
		}
	})

	t.Run("CreateNote", func(t *testing.T) {
		if notebookID == "" {
			t.Skip("depends on CreateNotebook")
		}
		note, err := client.CreateNote(noteTitle, noteBody, notebookID)
		if err != nil {
			t.Fatalf("CreateNote failed: %v", err)
		}
		if note.ID == "" {
			t.Fatal("expected non-empty note ID")
		}
		if note.Title != noteTitle {
			t.Errorf("expected title %q, got %q", noteTitle, note.Title)
		}
		noteID = note.ID
		t.Logf("Created note: %s (ID: %s)", note.Title, note.ID)
	})

	t.Run("ListNotes", func(t *testing.T) {
		if notebookID == "" || noteID == "" {
			t.Skip("depends on CreateNotebook + CreateNote")
		}
		notes, err := client.ListNotes(notebookID)
		if err != nil {
			t.Fatalf("ListNotes failed: %v", err)
		}
		found := false
		for _, n := range notes {
			if n.ID == noteID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("note %s not found in notebook %s (got %d notes)", noteID, notebookID, len(notes))
		}
	})

	t.Run("GetNote", func(t *testing.T) {
		if noteID == "" {
			t.Skip("depends on CreateNote")
		}
		note, err := client.GetNote(noteID)
		if err != nil {
			t.Fatalf("GetNote failed: %v", err)
		}
		if note.Title != noteTitle {
			t.Errorf("expected title %q, got %q", noteTitle, note.Title)
		}
		if note.Body != noteBody {
			t.Errorf("expected body %q, got %q", noteBody, note.Body)
		}
	})

	t.Run("AppendToNote", func(t *testing.T) {
		if noteID == "" {
			t.Skip("depends on CreateNote")
		}
		note, err := client.AppendToNote(noteID, appendContent)
		if err != nil {
			t.Fatalf("AppendToNote failed: %v", err)
		}

		// Re-fetch to verify
		fetched, err := client.GetNote(noteID)
		if err != nil {
			t.Fatalf("GetNote after append failed: %v", err)
		}
		if !strings.Contains(fetched.Body, noteBody) {
			t.Errorf("expected body to contain original %q, got %q", noteBody, fetched.Body)
		}
		if !strings.Contains(fetched.Body, appendContent) {
			t.Errorf("expected body to contain appended %q, got %q", appendContent, fetched.Body)
		}
		_ = note
	})

	t.Run("SearchNotes", func(t *testing.T) {
		if noteID == "" {
			t.Skip("depends on CreateNote")
		}
		notes, err := client.SearchNotes(noteTitle)
		if err != nil {
			t.Fatalf("SearchNotes failed: %v", err)
		}
		found := false
		for _, n := range notes {
			if n.ID == noteID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("note %s not found in search results for %q (got %d results)", noteID, noteTitle, len(notes))
		}
	})

	t.Run("CreateNestedNotebook", func(t *testing.T) {
		if notebookID == "" {
			t.Skip("depends on CreateNotebook")
		}
		child, err := client.CreateNotebook("e2e-child-notebook", notebookID)
		if err != nil {
			t.Fatalf("CreateNotebook (nested) failed: %v", err)
		}
		if child.ParentID != notebookID {
			t.Errorf("expected parent_id %q, got %q", notebookID, child.ParentID)
		}
		t.Logf("Created child notebook: %s (ID: %s, parent: %s)", child.Title, child.ID, child.ParentID)
	})

	// --- To-do flow ---

	t.Run("CreateTodo", func(t *testing.T) {
		if notebookID == "" {
			t.Skip("depends on CreateNotebook")
		}
		note, err := client.CreateTodo(todoTitle, todoBody, notebookID, 0)
		if err != nil {
			t.Fatalf("CreateTodo failed: %v", err)
		}
		if note.ID == "" {
			t.Fatal("expected non-empty todo ID")
		}
		if note.Title != todoTitle {
			t.Errorf("expected title %q, got %q", todoTitle, note.Title)
		}
		if note.IsTodo != 1 {
			t.Errorf("expected IsTodo 1, got %d", note.IsTodo)
		}
		todoID = note.ID
		t.Logf("Created todo: %s (ID: %s)", note.Title, note.ID)
	})

	t.Run("ListTodos", func(t *testing.T) {
		if notebookID == "" || todoID == "" {
			t.Skip("depends on CreateNotebook + CreateTodo")
		}
		todos, err := client.ListTodos(notebookID)
		if err != nil {
			t.Fatalf("ListTodos failed: %v", err)
		}
		found := false
		for _, td := range todos {
			if td.ID == todoID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("todo %s not found in ListTodos for notebook %s (got %d todos)", todoID, notebookID, len(todos))
		}
	})

	t.Run("MarkTodoCompleted", func(t *testing.T) {
		if todoID == "" {
			t.Skip("depends on CreateTodo")
		}
		note, err := client.MarkTodoCompleted(todoID)
		if err != nil {
			t.Fatalf("MarkTodoCompleted failed: %v", err)
		}
		if note.TodoCompleted == 0 {
			t.Error("expected TodoCompleted to be set (non-zero)")
		}
		t.Logf("Marked todo %s completed at %d", note.ID, note.TodoCompleted)
	})

	t.Run("GetNote_ShowsTodoCompleted", func(t *testing.T) {
		if todoID == "" {
			t.Skip("depends on CreateTodo")
		}
		note, err := client.GetNote(todoID)
		if err != nil {
			t.Fatalf("GetNote(todo) failed: %v", err)
		}
		if note.TodoCompleted == 0 {
			t.Error("expected todo to remain completed after GetNote")
		}
	})

	t.Run("MarkTodoUncompleted", func(t *testing.T) {
		if todoID == "" {
			t.Skip("depends on CreateTodo")
		}
		note, err := client.MarkTodoUncompleted(todoID)
		if err != nil {
			t.Fatalf("MarkTodoUncompleted failed: %v", err)
		}
		if note.TodoCompleted != 0 {
			t.Errorf("expected TodoCompleted 0 after uncomplete, got %d", note.TodoCompleted)
		}
	})
}
