# Context Key Best Practices in Go

## The Problem

Using plain strings as context keys can lead to collisions:

```go
// ❌ BAD: Plain string keys can collide
ctx := context.WithValue(r.Context(), "logger", logger)
ctx = context.WithValue(ctx, "user", user)
```

If any third-party library or another part of your codebase uses the same string key, they can accidentally overwrite each other's values.

## The Solution

Use a custom unexported type for context keys:

```go
// ✅ GOOD: Custom type prevents collisions
type contextKey string

const (
    LoggerContextKey contextKey = "logger"
    UserContextKey contextKey = "user"
)

ctx := context.WithValue(r.Context(), LoggerContextKey, logger)
ctx = context.WithValue(ctx, UserContextKey, user)
```

## Why This Works

1. **Type Safety**: `contextKey` is a distinct type from `string`, even though it's based on string
2. **Unexported**: The `contextKey` type is lowercase (unexported), so external packages cannot create values of this type
3. **No Collisions**: Even if another package uses the string `"logger"`, it won't collide because the types are different

## Example from Official Go Blog

From the [Go blog on context](https://go.dev/blog/context-keys-values):

```go
// User-defined key types should be unexported to avoid collisions.
type key int

const (
    userKey key = iota
    tokenKey
)

// WithUser returns a new Context that carries value u.
func WithUser(ctx context.Context, u *User) context.Context {
    return context.WithValue(ctx, userKey, u)
}

// User returns the User value stored in ctx, if any.
func User(ctx context.Context) (*User, bool) {
    u, ok := ctx.Value(userKey).(*User)
    return u, ok
}
```

## Our Implementation

In `internal/middleware/auth.go`:

```go
// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
    // UserContextKey is the key for storing user information in context
    UserContextKey contextKey = "user"
    // TraceIDContextKey is the key for storing trace ID in context
    TraceIDContextKey contextKey = "trace_id"
    // LoggerContextKey is the key for storing logger in context
    LoggerContextKey contextKey = "logger"
)
```

## Usage

### Setting Values

```go
// In middleware
ctx := context.WithValue(r.Context(), LoggerContextKey, logger)
ctx = context.WithValue(ctx, UserContextKey, user)
ctx = context.WithValue(ctx, TraceIDContextKey, traceID)
```

### Getting Values

```go
// In handlers
logger := middleware.LoggerFromContext(ctx)
user := middleware.GetUserFromContext(ctx)
traceID := middleware.GetTraceIDFromContext(ctx)
```

## Benefits

1. **Type Safety**: Compiler catches misuse
2. **No Collisions**: External packages cannot interfere
3. **Idiomatic Go**: Follows official Go recommendations
4. **Self-Documenting**: Exported constants make usage clear
5. **Testable**: Easy to mock in tests

## References

- [Go Blog: Context](https://go.dev/blog/context)
- [Go Blog: Context and Structs](https://go.dev/blog/context-and-structs)
- [Effective Go: Context](https://go.dev/doc/effective_go#context)
