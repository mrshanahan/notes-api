package notes

import "time"

type Note struct {
    ID          int64 `json:"id"`
    Title       string `json:"title"`
    CreatedOn   time.Time `json:"created_on"`
    UpdatedOn   time.Time `json:"updated_on"`
}
