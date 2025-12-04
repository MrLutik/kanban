# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git make

# Copy go mod files first for caching
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /kanban ./cmd/kanban

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates git

COPY --from=builder /kanban /usr/local/bin/kanban

ENTRYPOINT ["kanban"]
