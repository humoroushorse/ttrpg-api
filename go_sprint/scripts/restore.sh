#!/bin/bash
# Restore script for Sprint Management System
# Restores database from a backup file

set -e

# Configuration with defaults
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-sprint_management}"
DB_USER="${DB_USER:-postgres}"
BACKUP_FILE="$1"

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

# Check if backup file is provided
if [ -z "$BACKUP_FILE" ]; then
    log_error "Usage: $0 <backup_file>"
    log_error "Example: $0 ./backups/sprint_management_20240115_120000.sql"
    exit 1
fi

# Check if backup file exists
if [ ! -f "$BACKUP_FILE" ]; then
    log_error "Backup file not found: $BACKUP_FILE"
    exit 1
fi

# Check if psql is available
if ! command -v psql &> /dev/null; then
    log_error "psql not found. Please install PostgreSQL client tools."
    exit 1
fi

# Verify checksum if available
CHECKSUM_FILE="${BACKUP_FILE}.sha256"
if [ -f "$CHECKSUM_FILE" ]; then
    log_info "Verifying backup integrity..."
    if sha256sum -c "$CHECKSUM_FILE" > /dev/null 2>&1; then
        log_info "Checksum verification passed"
    else
        log_error "Checksum verification failed. Backup file may be corrupted."
        exit 1
    fi
else
    log_warn "No checksum file found. Skipping integrity check."
fi

# Confirm restore operation
log_warn "WARNING: This will restore the database from backup."
log_warn "Database: $DB_NAME@$DB_HOST:$DB_PORT"
log_warn "Backup file: $BACKUP_FILE"
log_warn "This operation may overwrite existing data."
read -p "Are you sure you want to continue? (yes/no): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    log_info "Restore cancelled by user"
    exit 0
fi

# Create a pre-restore backup
log_info "Creating pre-restore backup..."
PRE_RESTORE_BACKUP="./backups/pre_restore_$(date +%Y%m%d_%H%M%S).sql"
mkdir -p ./backups
if pg_dump \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="$DB_NAME" \
    --schema=sprint_management \
    --schema=auth \
    --file="$PRE_RESTORE_BACKUP" 2>&1 | grep -v "^$"; then
    log_info "Pre-restore backup created: $PRE_RESTORE_BACKUP"
else
    log_warn "Failed to create pre-restore backup. Continuing anyway..."
fi

# Restore database
log_info "Starting restore from: $BACKUP_FILE"

if psql \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="$DB_NAME" \
    --file="$BACKUP_FILE" 2>&1 | grep -v "^$"; then
    log_info "Restore completed successfully"
else
    log_error "Restore failed"
    log_error "You can restore from pre-restore backup: $PRE_RESTORE_BACKUP"
    exit 1
fi

# Verify restore
log_info "Verifying restore..."
TABLE_COUNT=$(psql \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="$DB_NAME" \
    --tuples-only \
    --command="SELECT COUNT(*) FROM information_schema.tables WHERE table_schema IN ('sprint_management', 'auth');" | tr -d ' ')

if [ "$TABLE_COUNT" -gt 0 ]; then
    log_info "Verification passed. Found $TABLE_COUNT tables."
else
    log_error "Verification failed. No tables found in restored schemas."
    exit 1
fi

log_info "Restore process completed successfully"
log_info "Pre-restore backup saved at: $PRE_RESTORE_BACKUP"
