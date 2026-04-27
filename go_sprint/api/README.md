# Sprint Management API

This directory contains the OpenAPI specifications and generated code for the Sprint Management API.

## Directory Structure

```
api/
├── openapi/              # OpenAPI 3.0 specifications
│   ├── main.yaml         # Main API specification (imports domain specs)
│   ├── workitems.yaml    # Work items domain specification
│   └── sprints.yaml      # Sprints domain specification
├── generated/            # Generated Go server code (auto-generated)
│   └── api.gen.go        # Generated types and server interfaces
├── frontend/             # Frontend integration assets (auto-generated)
│   └── api-types.ts      # TypeScript interface definitions
├── oapi-codegen.yaml     # Configuration for oapi-codegen
└── README.md             # This file
```

## OpenAPI Specifications

The API is defined using OpenAPI 3.0 specifications organized by domain:

### Main Specification (`main.yaml`)

The main specification file imports and combines all domain-specific specifications. It includes:
- API metadata (title, version, description)
- Server configurations
- Security schemes (JWT authentication)
- Path references to domain specifications

### Domain Specifications

#### Work Items (`workitems.yaml`)

Defines endpoints for work item management:
- CRUD operations for work items (epics, stories, defects)
- Dependency management
- Parent-child relationships
- Advanced search and filtering

**Key Endpoints:**
- `GET /api/v1/workitems` - List work items with filtering
- `POST /api/v1/workitems` - Create a new work item
- `GET /api/v1/workitems/{id}` - Get work item details
- `PUT /api/v1/workitems/{id}` - Update work item
- `DELETE /api/v1/workitems/{id}` - Soft delete work item
- `POST /api/v1/workitems/search` - Advanced search

#### Sprints (`sprints.yaml`)

Defines endpoints for sprint management:
- CRUD operations for sprints
- Sprint planning and capacity management
- Metrics and analytics
- Burndown charts and velocity tracking

**Key Endpoints:**
- `GET /api/v1/sprints` - List sprints
- `POST /api/v1/sprints` - Create a new sprint
- `GET /api/v1/sprints/{id}` - Get sprint details
- `POST /api/v1/sprints/{id}/close` - Close a sprint
- `GET /api/v1/sprints/{id}/metrics` - Get sprint metrics
- `GET /api/v1/sprints/{id}/burndown` - Get burndown data

## Code Generation

### Prerequisites

Install required tools:

```bash
# Install oapi-codegen for Go server code generation
make openapi-install-tools

# Or manually:
go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest

# Optional: Install tools for TypeScript and documentation
npm install -g openapi-typescript
npm install -g redoc-cli
npm install -g swagger-markdown
```

### Generate Code

Generate all code and documentation:

```bash
# Generate everything (Go code, TypeScript, docs)
make openapi-generate

# Or run the script directly
./scripts/generate-api.sh
```

This will generate:
1. **Go Server Code** (`generated/api.gen.go`)
   - Type definitions for all request/response models
   - Server interface definitions
   - Chi router integration code
   - Embedded OpenAPI specification

2. **TypeScript Interfaces** (`frontend/api-types.ts`)
   - Type-safe interfaces for Angular/React/Vue
   - Enum definitions
   - Request/response types

3. **API Documentation** (`docs/api/`)
   - HTML documentation (Redoc)
   - Markdown reference guide

### Using Generated Code

#### Go Server Implementation

The generated code provides interfaces that you need to implement:

```go
// internal/handlers/workitems/handler.go
package workitems

import (
    "net/http"
    "github.com/humoroushorse/go_sprint/api/generated"
)

type Handler struct {
    service *Service
}

// Implement the generated ServerInterface
func (h *Handler) ListWorkItems(w http.ResponseWriter, r *http.Request, params generated.ListWorkItemsParams) {
    // Your implementation here
}

func (h *Handler) CreateWorkItem(w http.ResponseWriter, r *http.Request) {
    // Your implementation here
}

// ... implement other methods
```

#### TypeScript/Angular Usage

Import and use the generated types:

```typescript
import { WorkItem, CreateWorkItemRequest, Sprint } from './api/frontend/api-types';

// Type-safe API calls
const createWorkItem = (request: CreateWorkItemRequest): Observable<WorkItem> => {
  return this.http.post<WorkItem>('/api/v1/workitems', request);
};
```

## API Design Principles

### 1. Domain-Driven Organization

The API is organized by business domains (work items, sprints) rather than technical layers. Each domain has its own OpenAPI specification file.

### 2. RESTful Design

- Use standard HTTP methods (GET, POST, PUT, DELETE)
- Resource-based URLs
- Proper HTTP status codes
- Consistent error responses

### 3. Cursor-Based Pagination

All list endpoints use cursor-based pagination for efficient data retrieval:

```json
{
  "items": [...],
  "pagination": {
    "next_cursor": "eyJpZCI6IjEyMyJ9",
    "has_more": true,
    "total_count": 150
  }
}
```

### 4. Consistent Error Handling

All errors follow a standard format:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input",
    "details": {},
    "validation_errors": [
      {
        "field": "title",
        "code": "REQUIRED",
        "message": "Title is required"
      }
    ]
  },
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T10:30:45Z"
}
```

### 5. JWT Authentication

All endpoints require JWT authentication via Keycloak:

```
Authorization: Bearer <jwt_token>
```

### 6. Trace ID Correlation

All requests should include a trace ID for debugging:

```
X-Trace-ID: 550e8400-e29b-41d4-a716-446655440000
```

## Validation Rules

### Work Items

- **Title**: Required, 1-255 characters
- **Description**: Required, minimum 1 character
- **Type**: Must be one of: epic, story, defect
- **Status**: Must be one of: todo, in_progress, in_review, done, blocked
- **Priority**: Must be one of: low, medium, high, critical
- **Story Points**: Optional, non-negative integer
- **Parent ID**: Optional, must reference existing work item
- **Sprint ID**: Optional, must reference existing sprint

### Sprints

- **Name**: Required, 1-255 characters
- **Start Date**: Required, valid date
- **End Date**: Required, must be after start date
- **Capacity Points**: Optional, non-negative integer
- **Status**: Must be one of: planned, active, completed, cancelled

## Testing the API

### Using curl

```bash
# Get JWT token from Keycloak
TOKEN="your-jwt-token"

# List work items
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8003/api/v1/workitems

# Create a work item
curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "type": "story",
       "title": "User login feature",
       "description": "Implement user authentication",
       "priority": "high"
     }' \
     http://localhost:8003/api/v1/workitems

# Get sprint metrics
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8003/api/v1/sprints/{sprint-id}/metrics
```

### Using Postman

1. Import the OpenAPI specification: `api/openapi/main.yaml`
2. Configure authentication with your Keycloak JWT token
3. Test endpoints using the generated collection

### Using Swagger UI

View and test the API interactively:

```bash
# Serve the OpenAPI spec with Swagger UI
docker run -p 8080:8080 \
  -e SWAGGER_JSON=/api/openapi/main.yaml \
  -v $(pwd)/api/openapi:/api/openapi \
  swaggerapi/swagger-ui
```

Then open: http://localhost:8080

## Updating the API

### Adding New Endpoints

1. **Update OpenAPI Specification**
   - Add new paths to the appropriate domain file (`workitems.yaml` or `sprints.yaml`)
   - Define request/response schemas
   - Add proper validation rules

2. **Regenerate Code**
   ```bash
   make openapi-generate
   ```

3. **Implement Handlers**
   - Implement the generated interface methods
   - Add business logic
   - Wire up to the router

4. **Update Documentation**
   - Update frontend integration guide if needed
   - Add examples to README

### Modifying Existing Endpoints

1. Update the OpenAPI specification
2. Regenerate code: `make openapi-generate`
3. Update handler implementations
4. Update tests
5. Update documentation

### Best Practices

- **Version Breaking Changes**: Use API versioning for breaking changes
- **Backward Compatibility**: Maintain backward compatibility when possible
- **Document Changes**: Update CHANGELOG and API documentation
- **Test Thoroughly**: Test all changes with integration tests
- **Review Generated Code**: Review generated code changes in PRs

## Troubleshooting

### Code Generation Fails

**Problem**: `oapi-codegen` command not found

**Solution**: Install oapi-codegen:
```bash
go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest
```

**Problem**: OpenAPI validation errors

**Solution**: Validate your OpenAPI spec:
```bash
# Install openapi-cli
npm install -g @redocly/cli

# Validate spec
openapi lint api/openapi/main.yaml
```

### TypeScript Generation Issues

**Problem**: `openapi-typescript` not found

**Solution**: Install globally:
```bash
npm install -g openapi-typescript
```

### Import/Reference Errors

**Problem**: Cannot resolve $ref references

**Solution**: Ensure all referenced files exist and paths are correct. Use relative paths from the main.yaml file.

## Additional Resources

- [OpenAPI 3.0 Specification](https://swagger.io/specification/)
- [oapi-codegen Documentation](https://github.com/deepmap/oapi-codegen)
- [Frontend Integration Guide](../docs/frontend-integration.md)
- [API Documentation](../docs/api/api-documentation.html)
- [NATS Subject Patterns](../docs/nats.md)

## Contributing

When contributing to the API:

1. Follow the existing OpenAPI structure and conventions
2. Add comprehensive descriptions to all endpoints and schemas
3. Include examples in the OpenAPI specification
4. Regenerate code after making changes
5. Update documentation
6. Add tests for new endpoints
7. Ensure backward compatibility or version appropriately
