#!/bin/bash
# Script to generate Go server code and TypeScript interfaces from OpenAPI specifications

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
API_DIR="$PROJECT_ROOT/api"
OPENAPI_DIR="$API_DIR/openapi"
GENERATED_DIR="$API_DIR/generated"
DOCS_DIR="$PROJECT_ROOT/docs/api"
FRONTEND_DIR="$API_DIR/frontend"

echo "==> Generating API code from OpenAPI specifications..."

# Create output directories
mkdir -p "$GENERATED_DIR"
mkdir -p "$DOCS_DIR"
mkdir -p "$FRONTEND_DIR"

# Check if oapi-codegen is installed
if ! command -v oapi-codegen &> /dev/null; then
    echo "Error: oapi-codegen is not installed"
    echo "Install it with: go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest"
    exit 1
fi

# Generate Go server code from main OpenAPI spec
echo "==> Generating Go server code..."
oapi-codegen \
    -package api \
    -generate types,chi-server \
    -o "$GENERATED_DIR/api.gen.go" \
    "$OPENAPI_DIR/combined.yaml"

echo "✓ Generated Go server code: $GENERATED_DIR/api.gen.go"

# Generate TypeScript interfaces if openapi-typescript is available
if command -v openapi-typescript &> /dev/null; then
    echo "==> Generating TypeScript interfaces..."
    
    # Generate from main spec
    openapi-typescript "$OPENAPI_DIR/main.yaml" \
        --output "$FRONTEND_DIR/api-types.ts"
    
    echo "✓ Generated TypeScript interfaces: $FRONTEND_DIR/api-types.ts"
else
    echo "⚠ openapi-typescript not found, skipping TypeScript generation"
    echo "  Install with: npm install -g openapi-typescript"
fi

# Generate API documentation using redoc-cli if available
if command -v redoc-cli &> /dev/null; then
    echo "==> Generating API documentation..."
    
    redoc-cli bundle "$OPENAPI_DIR/main.yaml" \
        --output "$DOCS_DIR/api-documentation.html" \
        --title "Sprint Management API Documentation"
    
    echo "✓ Generated API documentation: $DOCS_DIR/api-documentation.html"
else
    echo "⚠ redoc-cli not found, skipping HTML documentation generation"
    echo "  Install with: npm install -g redoc-cli"
fi

# Generate markdown documentation using swagger-markdown if available
if command -v swagger-markdown &> /dev/null; then
    echo "==> Generating Markdown documentation..."
    
    swagger-markdown -i "$OPENAPI_DIR/main.yaml" \
        -o "$DOCS_DIR/api-reference.md"
    
    echo "✓ Generated Markdown documentation: $DOCS_DIR/api-reference.md"
else
    echo "⚠ swagger-markdown not found, skipping Markdown documentation"
    echo "  Install with: npm install -g swagger-markdown"
fi

echo ""
echo "==> API generation complete!"
echo ""
echo "Generated files:"
echo "  - Go server code: $GENERATED_DIR/api.gen.go"
if [ -f "$FRONTEND_DIR/api-types.ts" ]; then
    echo "  - TypeScript types: $FRONTEND_DIR/api-types.ts"
fi
if [ -f "$DOCS_DIR/api-documentation.html" ]; then
    echo "  - HTML documentation: $DOCS_DIR/api-documentation.html"
fi
if [ -f "$DOCS_DIR/api-reference.md" ]; then
    echo "  - Markdown docs: $DOCS_DIR/api-reference.md"
fi
echo ""
echo "Next steps:"
echo "  1. Review generated code in $GENERATED_DIR"
echo "  2. Implement handler functions for API endpoints"
echo "  3. Use TypeScript types in Angular frontend"
echo ""
