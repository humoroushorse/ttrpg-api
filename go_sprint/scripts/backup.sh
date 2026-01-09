#!/bin/bash
# Backup script for Sprint Management System
# Creates a complete backup of the database including both schemas

set -e

# Configuration with defaults
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="${BACKUP_DIR:-./backups}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-sprint_management}"
DB_USER="${DB_USER:-postgres}"
BACKUP_TYPE="${BACKUP_TYPE:-full}"  # full, schema-only, data-only

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if pg_dump is available
if ! command -v pg_dump &> /dev/null; then
    log_error "pg_dump not found. Please install PostgreSQL client tools."
    exit 1
fi

# Create backup directory
mkdir -p "$BACKUP_DIR"
log_info "Backup directory: $BACKUP_DIR"

# Build backup filename
BACKUP_FILE="$BACKUP_DIR/sprint_management_${TIMESTAMP}.sql"

# Build pg_dump command
PG_DUMP_ARGS=(
    "--host=$DB_HOST"
    "--port=$DB_PORT"
    "--username=$DB_USER"
    "--dbname=$DB_NAME"
    "--schema=sprint_management"
    "--schema=auth"
    "--file=$BACKUP_FILE"
    "--verbose"
)

# Add backup type specific flags
case "$BACKUP_TYPE" in
    schema-only)
        PG_DUMP_ARGS+=("--schema-only")
        log_info "Creating schema-only backup"
        ;;
    data-only)
        PG_DUMP_ARGS+=("--data-only")
        log_info "Creating data-only backup"
        ;;
    full)
        log_info "Creating full backup (schema + data)"
        ;;
    *)
        log_error "Invalid backup type: $BACKUP_TYPE. Use: full, schema-only, or data-only"
        exit 1
        ;;
esac

# Create backup
log_info "Starting backup: $BACKUP_FILE"
log_info "Database: $DB_NAME@$DB_HOST:$DB_PORT"

if pg_dump "${PG_DUMP_ARGS[@]}" 2>&1 | grep -v "^$"; then
    # Validate backup
    if [ -f "$BACKUP_FILE" ]; then
        BACKUP_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
        log_info "Backup completed successfully"
        log_info "File: $BACKUP_FILE"
        log_info "Size: $BACKUP_SIZE"
        
        # Create checksum
        CHECKSUM_FILE="${BACKUP_FILE}.sha256"
        sha256sum "$BACKUP_FILE" > "$CHECKSUM_FILE"
        log_info "Checksum created: $CHECKSUM_FILE"
    else
        log_error "Backup file not created"
        exit 1
    fi
else
    log_error "Backup failed"
    exit 1
fi

# Cleanup old backups if retention is set
if [ -n "$RETENTION_DAYS" ] && [ "$RETENTION_DAYS" -gt 0 ]; then
    log_info "Cleaning up backups older than $RETENTION_DAYS days"
    find "$BACKUP_DIR" -name "sprint_management_*.sql" -mtime +"$RETENTION_DAYS" -delete
    find "$BACKUP_DIR" -name "sprint_management_*.sql.sha256" -mtime +"$RETENTION_DAYS" -delete
    log_info "Cleanup completed"
fi

log_info "Backup process completed successfully"
