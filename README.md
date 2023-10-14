# note-api

Service that provides a simple HTTP REST-ish API for creating, reading, editing, and searching notes.

## Building

Entirely Go-based; main entrypoint is in [`cmd/notes-api.go`](./cmd/notes-api.go).

Just `go run` that file to run the whole shebang:

    $ go run ./cmd/notes-api.go
    2023/10/14 17:14:13 INFO no valid port provided via NOTES_API_PORT, using default portStr="" port=3333
    2023/10/14 17:14:13 INFO listening for requests port=3333

You can provide a different port using the `NOTES_API_PORT` environment variable:

    $ NOTES_API_PORT=1111 go run ./cmd/notes-api.go 
    2023/10/14 17:14:39 INFO using custom port port=1111
    2023/10/14 17:14:39 INFO listening for requests port=1111

And obviously you can `go build` the same file to get the bin.

## Testing

:eyes:

## Structure

Repository structure follows standard Golang conventions; see: https://github.com/golang-standards/project-layout

## Owner

Matt Shanahan (mrshanahan11235@gmail.com)

License info [here](./LICENSE).
