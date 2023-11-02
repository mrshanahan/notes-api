package notes

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
    "unicode"
)

var (
    INDEX_ID_PATTERN = regexp.MustCompile(`^id:\s*(.*)`)
    INDEX_TITLE_PATTERN = regexp.MustCompile(`^title:\s*(.*)`)
    INDEX_PATH_PATTERN = regexp.MustCompile(`^path:\s*(.*)`)
    INDEX_CREATEDON_PATTERN = regexp.MustCompile(`^created_on:\s*(.*)`)
)

type IndexEntry struct {
    ID string
    Title string
    Path string
    CreatedOn time.Time
}

func GetNotesRoot() string {
    root := os.Getenv("NOTES_ROOT")
    if root == "" {
        root = filepath.Join(os.Getenv("HOME"), ".notes")
    }
    return root
}

func NewNote(title string, index []*IndexEntry) (*IndexEntry, []*IndexEntry) {
    root := GetNotesRoot()
    name := newNoteName()
    path := filepath.Join(root, name)
    f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0700)
    for err != nil && errors.Is(err, os.ErrExist) {
        name = newNoteName()
        path = filepath.Join(root, name)
        f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0700)
    }
    if err != nil {
        panic(err)
    }
    defer f.Close()

    id := getNextID(index)
    entry := &IndexEntry{
        ID: id,
        Title: title,
        Path: path,
        CreatedOn: time.Now().UTC(),
    }
    index = append(index, entry)

    return entry, index
}

func ImportNote(srcPath string) (*IndexEntry, error) {
    // TODO: Clean up dst file if import fails
    root := GetNotesRoot()
    name := newNoteName()
    dstPath := filepath.Join(root, name)
    dstf, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0700)
    for err != nil && errors.Is(err, os.ErrExist) {
        name = newNoteName()
        dstPath = filepath.Join(root, name)
        dstf, err = os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0700)
    }
    if err != nil {
        return nil, err
    }
    defer dstf.Close()

    srcf, err := os.OpenFile(srcPath, os.O_RDONLY, 0400)
    if err != nil {
        return nil, err
    }
    defer srcf.Close()

    err = copyFile(srcf, dstf)
    if err != nil {
        return nil, err
    }

    _, title := filepath.Split(srcPath)
    return &IndexEntry{
        ID: "",
        Title: title,
        Path: dstPath,
        CreatedOn: time.Now().UTC(),
    }, nil
}

func LoadIndex() ([]*IndexEntry, error) {
    f, err := openIndex()
    if err != nil {
        return nil, err
    }
    defer f.Close()

    entries := []*IndexEntry{}
    var entry *IndexEntry
    scanner, idx := bufio.NewScanner(f), 0
    if !scanner.Scan() {
        return nil, scanner.Err()
    }

    entry, more, linesProcessed, err := parseNextIndexEntry(scanner)
    for entry != nil && more && err == nil {
        entries = append(entries, entry)
        idx += linesProcessed
        entry, more, linesProcessed, err = parseNextIndexEntry(scanner)
    }
    if err != nil {
        return nil, err
    }
    return entries, nil
}

func SaveIndex(entries []*IndexEntry) error {
    f, err := openIndex()
    if err != nil {
        return err
    }
    defer f.Close()

    err = f.Truncate(0)
    if err != nil {
        return err
    }
    for _, e := range entries {
        f.WriteString(fmt.Sprintf("id: %s\n", e.ID))
        f.WriteString(fmt.Sprintf("title: %s\n", e.Title))
        f.WriteString(fmt.Sprintf("path: %s\n", e.Path))
        f.WriteString(fmt.Sprintf("created_on: %s\n", e.CreatedOn.Format(time.RFC3339)))
        f.WriteString("\n")
    }
    return nil
}

func DeleteNote(id string, index []*IndexEntry) ([]*IndexEntry, error) {
    entry := LookupNote(id, index)
    if entry == nil {
        return index, nil
    }

    err := os.Remove(entry.Path)
    if err != nil && !errors.Is(err, os.ErrNotExist) {
        return index, err
    }

    index = removeEntry(id, index)
    return index, nil
}

func LookupNote(id string, index []*IndexEntry) *IndexEntry {
    var found *IndexEntry = nil
    for _, note := range index {
        if note.ID == id {
            found = note
            break
        }
    }
    return found
}

func GetNoteContents(entry *IndexEntry) ([]byte, error) {
    content, err := os.ReadFile(entry.Path)
    if err != nil {
        return nil, err
    }
    return content, nil
}

func SetNoteContents(content []byte, entry *IndexEntry) error {
    err := os.WriteFile(entry.Path, content, 0666)
    return err
}


// Private

func copyFile(srcf, dstf *os.File) error {
    bufsize := 1024
    buf := make([]byte, bufsize)
    read, err := srcf.Read(buf)
    for read > 0 && err == nil {
        writebuf := buf[:read]
        _, err := dstf.Write(writebuf)
        if err != nil {
            return err
        }
        read, err = srcf.Read(buf)
    }

    if err != nil && !errors.Is(err, io.EOF) {
        return err
    }
    return nil
}

func newNoteName() string {
    x := time.Now().UnixMilli()
    return fmt.Sprintf("note%015d.txt", x)
}

func openIndex() (*os.File, error) {
    root := GetNotesRoot()
    if err := os.MkdirAll(root, 0700); err != nil {
        return nil, err
    }
    path := filepath.Join(root, "index.txt")

    f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0700)
    if err != nil {
        return nil, err
    }
    return f, nil
}

func fileExists(path string) bool {
    f, err := os.OpenFile(path, os.O_RDONLY, 0000)
    defer f.Close()
    return errors.Is(err, os.ErrExist)
}

// NB: Includes "".
func isWhitespace(s string) bool {
    for _, r := range s {
        if !unicode.IsSpace(r) {
            return false
        }
    }
    return len(s) == 0
}

func skipBlankLines(scanner *bufio.Scanner, scanFirst bool) (bool, int, string) {
    var more bool
    if scanFirst {
        more = scanner.Scan()
    } else {
        more = true
    }

    line, linesSkipped := scanner.Text(), 0
    for more && isWhitespace(line) {
        more = scanner.Scan()
        linesSkipped++
        if more {
            line = scanner.Text()
        }
    }
    return more, linesSkipped, line
}

// Assumes that the scanner is pointing to the first token.
func parseNextIndexEntry(scanner *bufio.Scanner) (*IndexEntry, bool, int, error) {
    more, linesSkipped, line := skipBlankLines(scanner, false)
    linesProcessed := linesSkipped
    if !more {
        err := scanner.Err()
        return nil, false, linesProcessed, err
    }

    var id string
    idRaw := line
    if matches := INDEX_ID_PATTERN.FindStringSubmatch(idRaw); matches != nil {
        id = strings.TrimSpace(matches[1])
    } else {
        return nil, false, linesProcessed, errors.New(fmt.Sprintf("Invalid id: %s", idRaw))
    }

    more, linesSkipped, line = skipBlankLines(scanner, true)
    linesProcessed += linesSkipped
    if !more {
        if err := scanner.Err(); err != nil {
            return nil, false, linesProcessed, err
        }
        return nil, false, linesProcessed, errors.New(fmt.Sprintf("No matching title for id: %s", id))
    }

    var title string
    titleRaw := line
    if matches := INDEX_TITLE_PATTERN.FindStringSubmatch(titleRaw); matches != nil {
        title = strings.TrimSpace(matches[1])
    } else {
        return nil, false, linesProcessed, errors.New(fmt.Sprintf("Invalid title string: %s", titleRaw))
    }

    more, linesSkipped, line = skipBlankLines(scanner, true)
    linesProcessed += linesSkipped
    if !more {
        if err := scanner.Err(); err != nil {
            return nil, false, linesProcessed, err
        }
        return nil, false, linesProcessed, errors.New(fmt.Sprintf("No matching path for title: %s", title))
    }

    var path string
    pathRaw := line
    if matches := INDEX_PATH_PATTERN.FindStringSubmatch(pathRaw); matches != nil {
        path = strings.TrimSpace(matches[1])
    } else {
        return nil, false, linesProcessed, errors.New(fmt.Sprintf("Invalid path string: %s", pathRaw))
    }

    var createdOn time.Time

    more, linesSkipped, line = skipBlankLines(scanner, true)
    linesProcessed += linesSkipped
    if !more {
        if err := scanner.Err(); err != nil {
            return nil, false, linesProcessed, err
        }
    } else {
        createdOnRaw := line
        if matches := INDEX_CREATEDON_PATTERN.FindStringSubmatch(createdOnRaw); matches != nil {
            createdOnStr := strings.TrimSpace(matches[1])
            parsed, err := time.Parse(time.RFC3339, createdOnStr)
            if err != nil {
                return nil, false, linesProcessed, err
            }
            createdOn = parsed
            more = scanner.Scan()
        }
    }

    return &IndexEntry{id, title, path, createdOn}, more, linesProcessed, nil
}

type FieldSchema struct {
     Name string
     Type string
     IsOptional bool
}

type ParserSchema struct {
    Fields []*FieldSchema
    FieldFormat string
}


// TODO: Struct for parsing the entry

func getNextID(index []*IndexEntry) string {
    max := 0
    for _, entry := range index {
        id, _ := strconv.Atoi(entry.ID)
        if max <= 0 || max < id {
            max = id
        }
    }
    return fmt.Sprintf("%d", max+1)
}

func removeEntry(id string, index []*IndexEntry) []*IndexEntry {
    var foundIdx int = -1
    for i, entry := range index {
        if entry.ID == id {
            foundIdx = i
            break
        }
    }
    if foundIdx >= 0 {
        index = append(index[:foundIdx], index[foundIdx+1:]...)
    }
    return index
}
