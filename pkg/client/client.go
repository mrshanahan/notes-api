package client

import (
    "encoding/json"
    "fmt"
    "io"
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
    resp, err := c.invoke("GET", "/notes/")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    respBytes, err := validateResponse(resp)
    if err != nil {
        return nil, err
    }

    var notes []*notes.Note
    if err := json.Unmarshal(respBytes, &notes); err != nil {
        return nil, fmt.Errorf("error JSON-decoding response body: %w", err)
    }

    return notes, nil
}

func (c *Client) CreateNote(title string) (*notes.Note, error) {
    encTitle, err := json.Marshal(title)
    if err != nil {
        return nil, fmt.Errorf("error JSON-encoding title: %w", err)
    }
    payload := fmt.Sprintf("{\"title\":%s}", encTitle)

    resp, err := c.invokeWithPayload("POST", "/notes/", "application/json", strings.NewReader(payload))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    respBytes, err := validateResponse(resp)
    if err != nil {
        return nil, err
    }

    var note *notes.Note
    if err := json.Unmarshal(respBytes, &note); err != nil {
        return nil, fmt.Errorf("error JSON-decoding response body: %w", err)
    }

    return note, nil
}

func (c *Client) GetNote(id int64) (*notes.Note, error) {
    urlPath := fmt.Sprintf("/notes/%d", id)
    resp, err := c.invoke("GET", urlPath)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    respBytes, err := validateResponse(resp)
    if err != nil {
        return nil, err
    }

    var note *notes.Note
    if err := json.Unmarshal(respBytes, &note); err != nil {
        return nil, fmt.Errorf("error JSON-decoding response body: %w", err)
    }

    return note, nil
}

func (c *Client) UpdateNote(id int64, title string) error {
    urlPath := fmt.Sprintf("/notes/%d", id)
    encTitle, err := json.Marshal(title)
    if err != nil {
        return fmt.Errorf("error JSON-encoding title: %w", err)
    }

    payload := fmt.Sprintf("{\"title\":%s}", encTitle)
    resp, err := c.invokeWithPayload("POST", urlPath, "application/json", strings.NewReader(payload))
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    _, err = validateResponse(resp)
    return err
}

func (c *Client) DeleteNote(id int64) error {
    urlPath := fmt.Sprintf("/notes/%d", id)

    resp, err := c.invoke("DELETE", urlPath)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    _, err = validateResponse(resp)
    return err
}

func GetNoteContent() {
}

func UpdateNoteContent() {
}

// Private functions

func (c *Client) invoke(method string, path string) (*http.Response, error) {
    requestUrl, err := url.JoinPath(c.URL, path)
    if err != nil {
        return nil, fmt.Errorf("error building URL path: %w", err)
    }

    req, err := http.NewRequest(method, requestUrl, nil)
    if err != nil {
        return nil, fmt.Errorf("error building API request: %w", err)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("error invoking API: %w", err)
    }
    return resp, nil
}

func (c *Client) invokeWithPayload(method string, path string, contentType string, body io.Reader) (*http.Response, error) {
    requestUrl, err := url.JoinPath(c.URL, path)
    if err != nil {
        return nil, fmt.Errorf("error building URL path: %w", err)
    }

    req, err := http.NewRequest(method, requestUrl, body)
    if err != nil {
        return nil, fmt.Errorf("error building API request: %w", err)
    }
    req.Header.Set("Content-Type", contentType)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("error invoking API: %w", err)
    }
    return resp, nil
}

func validateResponse(resp *http.Response) ([]byte, error) {
    respBytes, err := utils.ReadToEnd(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading response body: %w", err)
    }

    // TODO: Wider range here?
    if resp.StatusCode >= 400 {
        respStr := strings.TrimSpace(string(respBytes))
        return nil, fmt.Errorf("invalid status code: %d (response: %s)", resp.StatusCode, respStr)
    }

    return respBytes, nil
}

