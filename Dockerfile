# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binaries
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /bin/agentmgr ./cmd/agentmgr
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /bin/agentmgr-helper ./cmd/agentmgr-helper

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    git \
    npm \
    python3 \
    py3-pip

# Create non-root user
RUN adduser -D -h /home/agentmgr agentmgr
USER agentmgr
WORKDIR /home/agentmgr

# Copy binaries from builder
COPY --from=builder /bin/agentmgr /usr/local/bin/
COPY --from=builder /bin/agentmgr-helper /usr/local/bin/

# Copy catalog
COPY --chown=agentmgr:agentmgr catalog.json /home/agentmgr/.agentmgr/catalog.json

# Create data directories
RUN mkdir -p /home/agentmgr/.agentmgr/data \
    && mkdir -p /home/agentmgr/.agentmgr/cache \
    && mkdir -p /home/agentmgr/.agentmgr/logs

# Expose REST API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD agentmgr version || exit 1

# Default command
ENTRYPOINT ["agentmgr"]
CMD ["--help"]
