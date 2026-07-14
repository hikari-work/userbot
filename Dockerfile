# Stage 1: Build the userbot binary
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git build-base

# Copy go.mod and go.sum and download modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Run code generators and compile
RUN go generate ./...
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/userbot main.go

# Stage 2: Final lightweight runtime image
FROM alpine:latest

# Install runtime packages
RUN apk add --no-cache \
    git \
    tzdata \
    ffmpeg \
    python3 \
    curl \
    ca-certificates

# Install the latest yt-dlp binary
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/userbot /app/userbot

# Run userbot
ENTRYPOINT ["/app/userbot"]
