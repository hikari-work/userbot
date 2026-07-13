FROM golang:1.26-alpine

# Install basic system packages
RUN apk add --no-cache \
    git \
    tzdata \
    ffmpeg \
    python3 \
    curl \
    ca-certificates \
    build-base

# Download the latest yt-dlp binary
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the codebase
COPY . .

# Copy the runner script supporting auto-rebuild loop
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Run entrypoint script
ENTRYPOINT ["/entrypoint.sh"]
