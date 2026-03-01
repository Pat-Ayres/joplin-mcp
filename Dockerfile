FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o joplin-mcp ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /app/joplin-mcp /usr/local/bin/joplin-mcp

ENTRYPOINT ["joplin-mcp"]
