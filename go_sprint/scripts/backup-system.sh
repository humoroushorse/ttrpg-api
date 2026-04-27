#!/bin/bash
# Comprehensive System Backup Script
# Backs up database, configuration files, NATS configs, and documentation

set -e

# Configuration with defaults
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_ROOT="${BACKUP_ROOT:-./backups}"
BACKUP_DIR="$BACKUP_ROOT/system_backup_$TIMESTAMP"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-sprint_management}"
DB_USER="${DB_USER:-postgres}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_section() {
    echo -e "${BLUE}[====]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Create backup directory structure
create_backup_structure() {
    log_section "Creating backup directory structure"
    mkdir -p "$BACKUP_DIR"/{database,config,migrations,api,docs,nats}
    log_info "Backup directory: $BACKUP_DIR"
}

# Backup database
backup_database() {
    log_section "Backing up database"
    
    if ! command -v pg_dump &> /dev/null; then
        log_error "pg_dump not found. Skipping database backup."
        return 1
    fi
    
    DB_BACKUP_FILE="$BACKUP_DIR/database/sprint_management_${TIMESTAMP}.sql"
    
    log_info "Creating database backup..."
    if pg_dump \
        --host="$DB_HOST" \
        --port="$DB_PORT" \
        --username="$DB_USER" \
        --dbname="$DB_NAME" \
        --schema=sprint_management \
        --schema=auth \
        --file="$DB_BACKUP_FILE" 2>&1 | grep -v "^$"; then
        
        # Create checksum
        sha256sum "$DB_BACKUP_FILE" > "${DB_BACKUP_FILE}.sha256"
        
        DB_SIZE=$(du -h "$DB_BACKUP_FILE" | cut -f1)
        log_info "Database backup completed: $DB_SIZE"
    else
        log_error "Database backup failed"
        return 1
    fi
}

# Backup configuration files
backup_config() {
    log_section "Backing up configuration files"
    
    # Backup docker-compose
    if [ -f "docker-compose.yml" ]; then
        cp docker-compose.yml "$BACKUP_DIR/config/"
        log_info "Backed up: docker-compose.yml"
    fi
    
    # Backup Makefile
    if [ -f "Makefile" ]; then
        cp Makefile "$BACKUP_DIR/config/"
        log_info "Backed up: Makefile"
    fi
    
    # Backup sqlc config
    if [ -f "sqlc.yaml" ]; then
        cp sqlc.yaml "$BACKUP_DIR/config/"
        log_info "Backed up: sqlc.yaml"
    fi
    
    # Backup go.mod and go.sum
    if [ -f "go.mod" ]; then
        cp go.mod "$BACKUP_DIR/config/"
        log_info "Backed up: go.mod"
    fi
    if [ -f "go.sum" ]; then
        cp go.sum "$BACKUP_DIR/config/"
        log_info "Backed up: go.sum"
    fi
    
    # Backup Dockerfile
    if [ -f "Dockerfile" ]; then
        cp Dockerfile "$BACKUP_DIR/config/"
        log_info "Backed up: Dockerfile"
    fi
}

# Backup migrations
backup_migrations() {
    log_section "Backing up database migrations"
    
    if [ -d "migrations" ]; then
        cp -r migrations/* "$BACKUP_DIR/migrations/" 2>/dev/null || true
        MIGRATION_COUNT=$(find "$BACKUP_DIR/migrations" -type f | wc -l)
        log_info "Backed up $MIGRATION_COUNT migration files"
    else
        log_warn "No migrations directory found"
    fi
}

# Backup OpenAPI specifications
backup_api_specs() {
    log_section "Backing up OpenAPI specifications"
    
    if [ -d "api" ]; then
        cp -r api/* "$BACKUP_DIR/api/" 2>/dev/null || true
        SPEC_COUNT=$(find "$BACKUP_DIR/api" -name "*.yaml" -o -name "*.yml" | wc -l)
        log_info "Backed up $SPEC_COUNT API specification files"
    else
        log_warn "No api directory found"
    fi
}

# Backup documentation
backup_docs() {
    log_section "Backing up documentation"
    
    if [ -d "docs" ]; then
        cp -r docs/* "$BACKUP_DIR/docs/" 2>/dev/null || true
        DOC_COUNT=$(find "$BACKUP_DIR/docs" -type f | wc -l)
        log_info "Backed up $DOC_COUNT documentation files"
    else
        log_warn "No docs directory found"
    fi
    
    # Backup README
    if [ -f "README.md" ]; then
        cp README.md "$BACKUP_DIR/"
        log_info "Backed up: README.md"
    fi
}

# Backup NATS configuration
backup_nats_config() {
    log_section "Backing up NATS configuration"
    
    # Backup NATS documentation
    if [ -f "docs/nats.md" ]; then
        cp docs/nats.md "$BACKUP_DIR/nats/"
        log_info "Backed up: NATS documentation"
    fi
    
    # Backup NATS deployment configs from root
    if [ -d "../deploy/nats" ]; then
        cp -r ../deploy/nats/* "$BACKUP_DIR/nats/" 2>/dev/null || true
        log_info "Backed up: NATS deployment configuration"
    fi
    
    # Create NATS subject documentation
    cat > "$BACKUP_DIR/nats/subjects.txt" << EOF
# NATS Subject Patterns - Sprint Management Service
# Generated: $TIMESTAMP

## Sprint Service Subjects
sprint.{trace_id}.workitem.create.request
sprint.{trace_id}.workitem.create.response
sprint.{trace_id}.workitem.update.request
sprint.{trace_id}.workitem.update.response
sprint.{trace_id}.sprint.create.request
sprint.{trace_id}.sprint.create.response
sprint.{trace_id}.notification.workitem.created
sprint.{trace_id}.notification.workitem.updated

## Auth Service Subjects
auth.{trace_id}.validate.request
auth.{trace_id}.validate.response

## Wildcard Patterns for Tracing
sprint.{trace_id}.*.*.*
auth.{trace_id}.*.*
EOF
    log_info "Created: NATS subject documentation"
}

# Create backup manifest
create_manifest() {
    log_section "Creating backup manifest"
    
    cat > "$BACKUP_DIR/MANIFEST.txt" << EOF
Sprint Management System - Backup Manifest
==========================================
Backup Date: $(date)
Timestamp: $TIMESTAMP
Backup Directory: $BACKUP_DIR

Database Information:
- Host: $DB_HOST
- Port: $DB_PORT
- Database: $DB_NAME
- Schemas: sprint_management, auth

Contents:
- Database backup: database/sprint_management_${TIMESTAMP}.sql
- Configuration files: config/
- Database migrations: migrations/
- OpenAPI specifications: api/
- Documentation: docs/
- NATS configuration: nats/
- README: README.md

File Counts:
- Configuration files: $(find "$BACKUP_DIR/config" -type f 2>/dev/null | wc -l)
- Migration files: $(find "$BACKUP_DIR/migrations" -type f 2>/dev/null | wc -l)
- API specifications: $(find "$BACKUP_DIR/api" -type f 2>/dev/null | wc -l)
- Documentation files: $(find "$BACKUP_DIR/docs" -type f 2>/dev/null | wc -l)
- NATS files: $(find "$BACKUP_DIR/nats" -type f 2>/dev/null | wc -l)

Total Backup Size: $(du -sh "$BACKUP_DIR" | cut -f1)

Restore Instructions:
1. Review this manifest
2. Restore database: psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f database/sprint_management_${TIMESTAMP}.sql
3. Copy configuration files back to project root
4. Run migrations if needed: make migrate-up
5. Verify NATS configuration matches nats/ directory

Notes:
- Verify database backup checksum before restore
- Ensure all dependencies are installed
- Review configuration files for environment-specific settings
EOF
    
    log_info "Manifest created: $BACKUP_DIR/MANIFEST.txt"
}

# Create compressed archive
create_archive() {
    log_section "Creating compressed archive"
    
    ARCHIVE_FILE="$BACKUP_ROOT/system_backup_${TIMESTAMP}.tar.gz"
    
    tar -czf "$ARCHIVE_FILE" -C "$BACKUP_ROOT" "system_backup_$TIMESTAMP"
    
    ARCHIVE_SIZE=$(du -h "$ARCHIVE_FILE" | cut -f1)
    log_info "Archive created: $ARCHIVE_FILE ($ARCHIVE_SIZE)"
    
    # Create checksum for archive
    sha256sum "$ARCHIVE_FILE" > "${ARCHIVE_FILE}.sha256"
    log_info "Archive checksum created"
}

# Main execution
main() {
    log_section "Starting Comprehensive System Backup"
    log_info "Timestamp: $TIMESTAMP"
    
    create_backup_structure
    backup_database
    backup_config
    backup_migrations
    backup_api_specs
    backup_docs
    backup_nats_config
    create_manifest
    create_archive
    
    log_section "Backup Completed Successfully"
    log_info "Backup location: $BACKUP_DIR"
    log_info "Archive: $BACKUP_ROOT/system_backup_${TIMESTAMP}.tar.gz"
    log_info "Review manifest: $BACKUP_DIR/MANIFEST.txt"
}

# Run main function
main
