package notes

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "time"
    "unicode"
)

var (
    INDEX_ID_PATTERN = regexp.MustCompile(`^id:\s*(.*)`)
    INDEX_TITLE_PATTERN = regexp.MustCompile(`^title:\s*(.*)`)
    INDEX_PATH_PATTERN = regexp.MustCompile(`^path:\s*(.*)`)
)

type IndexEntry struct {
    ID string
    Title string
    Path string
}

func GetNotesRoot() string {
    root := os.Getenv("NOTES_ROOT")
    if root == "" {
        root = filepath.Join(os.Getenv("HOME"), ".notes")
    }
    return root
}

func NewNote(title string) *IndexEntry {
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

    return &IndexEntry{
        ID: "",
        Title: title,
        Path: path,
    }
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
    entry, linesProcessed, err := parseNextIndexEntry(scanner)
    for entry != nil && err == nil {
        entries = append(entries, entry)
        idx += linesProcessed
        entry, linesProcessed, err = parseNextIndexEntry(scanner)
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
        f.WriteString(fmt.Sprintf("title: %s\n", e.Title))
        f.WriteString(fmt.Sprintf("path: %s\n", e.Path))
        f.WriteString("\n")
    }
    return nil
}

func DeleteNote(entry *IndexEntry) error {
    err := os.Remove(entry.Path)
    if err != nil && !errors.Is(err, os.ErrNotExist) {
        return err
    }
    return nil
}

func GetNoteContents(entry *IndexEntry) ([]byte, error) {
    content, err := os.ReadFile(entry.Path)
    if err != nil {
        return nil, err
    }
    return content, nil
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

func skipBlankLines(scanner *bufio.Scanner) (bool, int, string) {
    more, linesProcessed, line := scanner.Scan(), 1, scanner.Text()
    for more && isWhitespace(line) {
        more = scanner.Scan()
        linesProcessed++
        if more {
            line = scanner.Text()
        }
    }
    return more, linesProcessed, line
}

func parseNextIndexEntry(scanner *bufio.Scanner) (*IndexEntry, int, error) {
    more, linesSkipped, line := skipBlankLines(scanner)
    linesProcessed := linesSkipped
    if !more {
        err := scanner.Err()
        return nil, linesProcessed, err
    }

    var id string
    idRaw := line
    if matches := INDEX_ID_PATTERN.FindStringSubmatch(idRaw); matches != nil {
        id = strings.TrimSpace(matches[1])
    } else {
        return nil, linesProcessed, errors.New(fmt.Sprintf("Invalid id: %s", idRaw))
    }

    more, linesSkipped, line = skipBlankLines(scanner)
    linesProcessed += linesSkipped
    if !more {
        if err := scanner.Err(); err != nil {
            return nil, linesProcessed, err
        }
        return nil, linesProcessed, errors.New(fmt.Sprintf("No matching title for id: %s", id))
    }

    var title string
    titleRaw := line
    if matches := INDEX_TITLE_PATTERN.FindStringSubmatch(titleRaw); matches != nil {
        title = strings.TrimSpace(matches[1])
    } else {
        return nil, linesProcessed, errors.New(fmt.Sprintf("Invalid title string: %s", titleRaw))
    }

    more, linesSkipped, line = skipBlankLines(scanner)
    linesProcessed += linesSkipped
    if !more {
        if err := scanner.Err(); err != nil {
            return nil, linesProcessed, err
        }
        return nil, linesProcessed, errors.New(fmt.Sprintf("No matching path for title: %s", title))
    }

    var path string
    pathRaw := line
    if matches := INDEX_PATH_PATTERN.FindStringSubmatch(pathRaw); matches != nil {
        path = strings.TrimSpace(matches[1])
    } else {
        return nil, linesProcessed, errors.New(fmt.Sprintf("Invalid path string: %s", pathRaw))
    }

    return &IndexEntry{id, title, path}, linesProcessed, nil
}
