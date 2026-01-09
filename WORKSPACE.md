# Go Workspace Configuration

This repository uses Go workspaces (Go 1.18+) to manage multiple Go modules for local development.

## Overview

The workspace includes two Go modules:

1. **go_auth** - Authentication service and shared library
2. **go_sprint** - Sprint management service

## Workspace Structure

```
.
├── go.work              # Workspace configuration
├── go_auth/
│   ├── go.mod           # Auth service module
│   ├── pkg/auth/
│   │   └── go.mod       # Shared auth library module
│   └── ...
└── go_sprint/
    ├── go.mod           # Sprint service module (imports go_auth/pkg/auth)
    └── ...
```

## Local Development Setup

### Option 1: Go Workspaces (Recommended)

The `go.work` file at the root configures the workspace:

```go
go 1.21

use (
    ./go_auth
    ./go_sprint
)
```

With this setup:
- Both modules can reference each other locally
- No need for replace directives
- Changes in go_auth are immediately available in go_sprint
- Works seamlessly with Go tools (go build, go test, gopls)

### Option 2: Replace Directives

If not using workspaces, add replace directives to `go_sprint/go.mod`:

```go
replace github.com/humoroushorse/go_auth/pkg/auth => ../go_auth/pkg/auth
```

## Working with the Workspace

### Building Services

```bash
# Build auth service
cd go_auth
go build -o bin/auth-server cmd/server/main.go

# Build sprint service
cd go_sprint
go build -o bin/sprint-server cmd/server/main.go
```

### Running Tests

```bash
# Test all modules in workspace
go test ./...

# Test specific module
cd go_auth
go test ./...

cd go_sprint
go test ./...
```

### Adding Dependencies

```bash
# Add dependency to auth service
cd go_auth
go get github.com/some/package

# Add dependency to sprint service
cd go_sprint
go get github.com/some/package
```

### Syncing Workspace

```bash
# Sync all modules in workspace
go work sync
```

## Shared Library Usage

The `go_auth/pkg/auth` package is a shared library that can be imported by other services:

```go
// In go_sprint service
import (
    "github.com/humoroushorse/go_auth/pkg/auth/middleware"
    "github.com/humoroushorse/go_auth/pkg/auth/models"
)

func setupMiddleware(router *http.ServeMux, authClient *client.AuthClient) {
    // Use shared JWT middleware
    router.Use(middleware.JWTAuth(authClient))
}
```

## IDE Support

### VS Code

The Go extension automatically detects and uses the workspace configuration. Ensure you have:

```json
{
    "go.useLanguageServer": true,
    "gopls": {
        "experimentalWorkspaceModule": true
    }
}
```

### GoLand / IntelliJ IDEA

GoLand automatically detects `go.work` files and configures the workspace.

## CI/CD Considerations

### GitHub Actions

```yaml
- name: Setup Go
  uses: actions/setup-go@v4
  with:
    go-version: '1.21'

- name: Build and Test
  run: |
    go work sync
    go test ./...
    cd go_auth && go build ./...
    cd ../go_sprint && go build ./...
```

### Docker Builds

For Docker builds, you may need to copy both modules:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /workspace

# Copy workspace configuration
COPY go.work go.work

# Copy both modules
COPY go_auth/ go_auth/
COPY go_sprint/ go_sprint/

# Build specific service
WORKDIR /workspace/go_sprint
RUN go build -o /app/server cmd/server/main.go
```

## Publishing the Shared Library

When ready to publish `go_auth/pkg/auth` as a standalone module:

1. Tag the auth package:
   ```bash
   cd go_auth
   git tag pkg/auth/v0.1.0
   git push origin pkg/auth/v0.1.0
   ```

2. Update `go_sprint/go.mod` to use the published version:
   ```go
   require (
       github.com/humoroushorse/go_auth/pkg/auth v0.1.0
   )
   ```

3. Remove the replace directive for production builds

## Troubleshooting

### Module not found errors

If you see "module not found" errors:

```bash
# Sync workspace
go work sync

# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download
```

### IDE not recognizing imports

1. Restart the Go language server
2. Ensure `go.work` is in the workspace root
3. Check that both modules are listed in `go.work`

### Replace directive conflicts

If using both workspace and replace directives, the workspace takes precedence. Remove replace directives when using workspaces.

## Best Practices

1. **Commit go.work**: Include `go.work` in version control for consistent local development
2. **Version shared library**: Tag releases of `pkg/auth` for production use
3. **Test in isolation**: Periodically test modules independently to ensure they work without workspace
4. **Document dependencies**: Keep README files updated with module dependencies
5. **Use semantic versioning**: Follow semver for shared library releases

## References

- [Go Workspaces Documentation](https://go.dev/doc/tutorial/workspaces)
- [Go Modules Reference](https://go.dev/ref/mod)
- [Multi-module Repositories](https://github.com/golang/go/wiki/Modules#faqs--multi-module-repositories)
