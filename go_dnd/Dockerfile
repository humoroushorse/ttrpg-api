FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /workspace

COPY go.work go.work.sum* ./
COPY go_auth go_auth
COPY go_dnd go_dnd

WORKDIR /workspace/go_dnd

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/bin/dnd-server \
    cmd/server/main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/bin/migrate \
    cmd/migrate/main.go

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates

RUN addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/bin/dnd-server /app/dnd-server
COPY --from=builder /build/bin/migrate /app/migrate
COPY --from=builder /workspace/go_dnd/migrations /app/migrations

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8004

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8004/health/live || exit 1

ENTRYPOINT ["/app/dnd-server"]
