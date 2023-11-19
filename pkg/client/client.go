package client

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"

    "mrshanahan.com/notes-api/pkg/notes"
    "mrshanahan.com/notes-api/internal/utils"
)

type Client struct {
    URL string
}

func NewClient(url string) *Client {
    return &Client{url}
}

func (c *Client) ListNotes() ([]*notes.Note, error) {
    listUrl, err := url.JoinPath(c.URL, "/notes/")
    if err != nil {
        return nil, fmt.Errorf("error building URL path: %w", err)
    }
    resp, err := http.Get(listUrl)
    if err != nil {
        return nil, fmt.Errorf("error invoking API: %w", err)
    }
    defer resp.Body.Close()

    respBytes, err := utils.ReadToEnd(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading response body: %w", err)
    }

    var notes []*notes.Note
    if err := json.Unmarshal(respBytes, &notes); err != nil {
        return nil, fmt.Errorf("error JSON-decoding response body: %w", err)
    }

    return notes, nil
}

func (c *Client) CreateNote(title string) (*notes.Note, error) {
    postUrl, err := url.JoinPath(c.URL, "/notes/")
    if err != nil {
        return nil, fmt.Errorf("error building URL path: %w", err)
    }

    encTitle, err := json.Marshal(title)
    if err != nil {
        return nil, fmt.Errorf("error JSON-encoding title: %w", err)
    }

    payload := fmt.Sprintf("{\"title\":%s}", encTitle)
    resp, err := http.Post(postUrl, "application/json", strings.NewReader(payload))
    if err != nil {
        return nil, fmt.Errorf("error invoking API: %w", err)
    }
    defer resp.Body.Close()

    respBytes, err := utils.ReadToEnd(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading response body: %w", err)
    }

    var note *notes.Note
    if err := json.Unmarshal(respBytes, &note); err != nil {
        return nil, fmt.Errorf("error JSON-decoding response body: %w", err)
    }

    return note, nil
}

func (c *Client) GetNote(id int) (*notes.Note, error) {
    urlPath := fmt.Sprintf("/notes/%d", id)
    getUrl, err := url.JoinPath(c.URL, urlPath)
    if err != nil {
        return nil, fmt.Errorf("error building URL path: %w", err)
    }
    resp, err := http.Get(getUrl)
    if err != nil {
        return nil, fmt.Errorf("error invoking API: %w", err)
    }
    defer resp.Body.Close()

    respBytes, err := utils.ReadToEnd(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading response body: %w", err)
    }

    // TODO: Wider range here?
    if resp.StatusCode >= 400 {
        respStr := strings.TrimSpace(string(respBytes))
        return nil, fmt.Errorf("invalid status code: %d (response: %s)", resp.StatusCode, respStr)
    }

    var note *notes.Note
    if err := json.Unmarshal(respBytes, &note); err != nil {
        return nil, fmt.Errorf("error JSON-decoding response body: %w", err)
    }

    return note, nil
}

func (c *Client) UpdateNote(id int64, title string) error {
    urlPath := fmt.Sprintf("/notes/%d", id)
    updateUrl, err := url.JoinPath(c.URL, urlPath)
    if err != nil {
        return fmt.Errorf("error building URL path: %w", err)
    }

    encTitle, err := json.Marshal(title)
    if err != nil {
        return fmt.Errorf("error JSON-encoding title: %w", err)
    }

    payload := fmt.Sprintf("{\"title\":%s}", encTitle)
    resp, err := http.Post(updateUrl, "application/json", strings.NewReader(payload))
    if err != nil {
        return fmt.Errorf("error invoking API: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode < 400 {
        return nil
    }

    respBytes, err := utils.ReadToEnd(resp.Body)
    if err != nil {
        return fmt.Errorf("error reading response body: %w", err)
    }

    respStr := strings.TrimSpace(string(respBytes))
    return fmt.Errorf("invalid status code: %d (response: %s)", resp.StatusCode, respStr)
}

func DeleteNote() {
}

func GetNoteContent() {
}

func UpdateNoteContent() {
}
