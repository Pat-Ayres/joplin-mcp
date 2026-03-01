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

// Note represents a Joplin note.
type Note struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Title    string `json:"title"`
	Body     string `json:"body,omitempty"`
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

// GetNote fetches a note by ID, including its body.
func (c *Client) GetNote(noteID string) (*Note, error) {
	data, err := c.doRequest("GET", fmt.Sprintf("/notes/%s?fields=id,parent_id,title,body", noteID), nil)
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
