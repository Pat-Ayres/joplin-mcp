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
	const (
		notebookTitle = "e2e-test-notebook"
		noteTitle     = "e2e-test-note"
		noteBody      = "This is the original body."
		appendContent = "This was appended."
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
}
