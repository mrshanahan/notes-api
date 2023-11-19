package utils

import (
    "errors"
    "io"
)

func ReadToEnd(r io.Reader) ([]byte, error) {
    BUF_SIZE := 1024 * 8
    buffer := make([]byte, BUF_SIZE)
    result := []byte{}
    readMore := true
    var err error = nil
    for readMore {
        numRead, err := r.Read(buffer)
        if err != nil && !errors.Is(err, io.EOF) {
            readMore = false
        } else {
            readMore = err == nil
            result = append(result, buffer[:numRead]...)
        }
    }
    if err != nil {
        return nil, err
    }
    return result, nil
}
