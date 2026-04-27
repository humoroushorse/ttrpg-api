# Multi-stage Dockerfile for go_sprint service

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /workspace

# Copy workspace files
COPY go.work go.work.sum* ./

# Copy go_auth module (dependency)
COPY go_auth go_auth

# Copy go_sprint
COPY go_sprint go_sprint

# Set working directory to go_sprint
WORKDIR /workspace/go_sprint

# Download dependencies
RUN go mod download

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/bin/sprint-server \
    cmd/server/main.go

# Build the migrate binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/bin/migrate \
    cmd/migrate/main.go

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/bin/sprint-server /app/sprint-server
COPY --from=builder /build/bin/migrate /app/migrate

# Copy migrations
COPY --from=builder /workspace/go_sprint/migrations /app/migrations

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health/live || exit 1

# Run the binary
ENTRYPOINT ["/app/sprint-server"]
