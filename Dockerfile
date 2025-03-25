FROM golang:latest as builder

RUN mkdir -p /app
COPY . /app/notes-api
WORKDIR /app/notes-api
RUN go build ./cmd/notes-api.go

# NB: I tried to use alpine here but I would get "exec /app/notes-api: no such file or directory" when attempting
# to run the exe. The same would be true when running the container directly & invoking it, despite the fact that
# the file was discoverable by ls. Not sure why but that doesn't happen on ubuntu.
FROM ubuntu:latest

# Need to install ca-certificates explicitly to get the LetsEncrypt root cert
RUN apt update
RUN apt install ca-certificates -y
RUN mkdir -p /app
COPY --from=builder /app/notes-api/notes-api /app/notes-api
RUN mkdir -p /app/data

ENV NOTES_API_PORT 3333
ENV NOTES_API_DB_DIR /app/data
ENV NOTES_API_AUTH_PROVIDER_URL http://localhost:8080/realms/myrealm
ENV NOTES_API_REDIRECT_URL http://localhost:3333/notes/auth/callback

ENTRYPOINT [ "/app/notes-api" ]