package joplin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// JoplinClient defines the interface for interacting with the Joplin API.
type JoplinClient interface {
	ListNotebooks() ([]Notebook, error)
	ListNotes(notebookID string) ([]Note, error)
	GetNote(noteID string) (*Note, error)
	CreateNote(title, body, notebookID string) (*Note, error)
	AppendToNote(noteID, content string) (*Note, error)
	SearchNotes(query string) ([]Note, error)
	CreateNotebook(title, parentID string) (*Notebook, error)
	CreateTodo(title, body, notebookID string, dueUnixMs int64) (*Note, error)
	MarkTodoCompleted(noteID string) (*Note, error)
	MarkTodoUncompleted(noteID string) (*Note, error)
	ListTodos(notebookID string) ([]Note, error)
}

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

// Note represents a Joplin note.
type Note struct {
	ID            string `json:"id"`
	ParentID      string `json:"parent_id"`
	Title         string `json:"title"`
	Body          string `json:"body,omitempty"`
	IsTodo        int    `json:"is_todo,omitempty"`
	TodoDue       int64  `json:"todo_due,omitempty"`
	TodoCompleted int64  `json:"todo_completed,omitempty"`
}

// paginatedResponse is used to decode paginated Joplin API responses.
type paginatedResponse struct {
	Items   json.RawMessage `json:"items"`
	HasMore bool            `json:"has_more"`
}

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

// ListNotebooks returns all notebooks.
func (c *Client) ListNotebooks() ([]Notebook, error) {
	rawItems, err := c.fetchAllPages("/folders", nil)
	if err != nil {
		return nil, fmt.Errorf("listing notebooks: %w", err)
	}

	var notebooks []Notebook
	for _, raw := range rawItems {
		var nb Notebook
		if err := json.Unmarshal(raw, &nb); err != nil {
			return nil, fmt.Errorf("decoding notebook: %w", err)
		}
		notebooks = append(notebooks, nb)
	}
	return notebooks, nil
}

// ListNotes returns all notes in a given notebook.
func (c *Client) ListNotes(notebookID string) ([]Note, error) {
	params := url.Values{}
	params.Set("fields", "id,parent_id,title")

	rawItems, err := c.fetchAllPages(fmt.Sprintf("/folders/%s/notes", notebookID), params)
	if err != nil {
		return nil, fmt.Errorf("listing notes: %w", err)
	}

	var notes []Note
	for _, raw := range rawItems {
		var n Note
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, fmt.Errorf("decoding note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, nil
}

// GetNote fetches a note by ID, including its body and todo fields.
func (c *Client) GetNote(noteID string) (*Note, error) {
	data, err := c.doRequest("GET", fmt.Sprintf("/notes/%s?fields=id,parent_id,title,body,is_todo,todo_due,todo_completed", noteID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting note: %w", err)
	}

	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		return nil, fmt.Errorf("decoding note: %w", err)
	}
	return &note, nil
}

// CreateNote creates a new note in the specified notebook.
func (c *Client) CreateNote(title, body, notebookID string) (*Note, error) {
	payload := map[string]string{
		"title":     title,
		"body":      body,
		"parent_id": notebookID,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding note: %w", err)
	}

	data, err := c.doRequest("POST", "/notes", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("creating note: %w", err)
	}

	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		return nil, fmt.Errorf("decoding note: %w", err)
	}
	return &note, nil
}

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

// SearchNotes performs a full-text search across all notes.
func (c *Client) SearchNotes(query string) ([]Note, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("fields", "id,parent_id,title")

	rawItems, err := c.fetchAllPages("/search", params)
	if err != nil {
		return nil, fmt.Errorf("searching notes: %w", err)
	}

	var notes []Note
	for _, raw := range rawItems {
		var n Note
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, fmt.Errorf("decoding note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, nil
}

// CreateNotebook creates a new notebook (folder).
func (c *Client) CreateNotebook(title, parentID string) (*Notebook, error) {
	payload := map[string]string{
		"title": title,
	}
	if parentID != "" {
		payload["parent_id"] = parentID
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding notebook: %w", err)
	}

	data, err := c.doRequest("POST", "/folders", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("creating notebook: %w", err)
	}

	var nb Notebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return nil, fmt.Errorf("decoding notebook: %w", err)
	}
	return &nb, nil
}

// CreateTodo creates a new to-do note in the specified notebook.
// If dueUnixMs > 0, the todo will have a due date set.
func (c *Client) CreateTodo(title, body, notebookID string, dueUnixMs int64) (*Note, error) {
	payload := map[string]any{
		"title":     title,
		"body":      body,
		"parent_id": notebookID,
		"is_todo":   1,
	}
	if dueUnixMs > 0 {
		payload["todo_due"] = dueUnixMs
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding todo: %w", err)
	}

	data, err := c.doRequest("POST", "/notes", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("creating todo: %w", err)
	}

	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		return nil, fmt.Errorf("decoding note: %w", err)
	}
	return &note, nil
}

// MarkTodoCompleted marks a to-do note as completed (sets todo_completed to current time in ms).
func (c *Client) MarkTodoCompleted(noteID string) (*Note, error) {
	completedMs := time.Now().UnixMilli()
	payload := map[string]int64{
		"todo_completed": completedMs,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding update: %w", err)
	}

	data, err := c.doRequest("PUT", fmt.Sprintf("/notes/%s", noteID), strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("marking todo completed: %w", err)
	}

	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		return nil, fmt.Errorf("decoding note: %w", err)
	}
	return &note, nil
}

// MarkTodoUncompleted marks a to-do note as not completed (sets todo_completed to 0).
func (c *Client) MarkTodoUncompleted(noteID string) (*Note, error) {
	payload := map[string]int64{
		"todo_completed": 0,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding update: %w", err)
	}

	data, err := c.doRequest("PUT", fmt.Sprintf("/notes/%s", noteID), strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("marking todo uncompleted: %w", err)
	}

	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		return nil, fmt.Errorf("decoding note: %w", err)
	}
	return &note, nil
}

// noteFieldsWithTodo is the fields query for note responses that include todo state.
const noteFieldsWithTodo = "id,parent_id,title,body,is_todo,todo_due,todo_completed"

// ListTodos returns to-do notes, optionally scoped to a notebook.
// If notebookID is empty, returns all todos; otherwise only todos in that notebook.
func (c *Client) ListTodos(notebookID string) ([]Note, error) {
	params := url.Values{}
	params.Set("fields", noteFieldsWithTodo)

	var rawItems []json.RawMessage
	var err error
	if notebookID != "" {
		rawItems, err = c.fetchAllPages(fmt.Sprintf("/folders/%s/notes", notebookID), params)
	} else {
		rawItems, err = c.fetchAllPages("/notes", params)
	}
	if err != nil {
		return nil, fmt.Errorf("listing todos: %w", err)
	}

	var notes []Note
	for _, raw := range rawItems {
		var n Note
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, fmt.Errorf("decoding note: %w", err)
		}
		if n.IsTodo == 1 {
			notes = append(notes, n)
		}
	}
	return notes, nil
}
