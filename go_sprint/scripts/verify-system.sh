#!/bin/bash
# System Verification Script
# Verifies system health after restoration or deployment

set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-sprint_management}"
DB_USER="${DB_USER:-postgres}"
API_URL="${API_URL:-http://localhost:8080}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
PASSED=0
FAILED=0
WARNINGS=0

# Functions
log_info() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_fail() {
    echo -e "${RED}[✗]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[!]${NC} $1"
}

log_section() {
    echo -e "\n${BLUE}[====]${NC} $1"
}

check_pass() {
    ((PASSED++))
    log_info "$1"
}

check_fail() {
    ((FAILED++))
    log_fail "$1"
}

check_warn() {
    ((WARNINGS++))
    log_warn "$1"
}

# Database checks
check_database() {
    log_section "Database Verification"
    
    # Check PostgreSQL connectivity
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
        check_pass "Database connection successful"
    else
        check_fail "Database connection failed"
        return 1
    fi
    
    # Check schemas exist
    SCHEMA_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c \
        "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name IN ('sprint_management', 'auth');")
    
    if [ "$SCHEMA_COUNT" -eq 2 ]; then
        check_pass "Required schemas exist (sprint_management, auth)"
    else
        check_fail "Missing required schemas (found: $SCHEMA_COUNT, expected: 2)"
    fi
    
    # Check core tables
    TABLES=(
        "sprint_management.work_items"
        "sprint_management.sprints"
        "sprint_management.work_item_dependencies"
        "sprint_management.comments"
        "sprint_management.activity_logs"
        "auth.users"
    )
    
    for table in "${TABLES[@]}"; do
        if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c \
            "SELECT 1 FROM information_schema.tables WHERE table_schema='${table%%.*}' AND table_name='${table##*.}';" | grep -q 1; then
            check_pass "Table exists: $table"
        else
            check_fail "Table missing: $table"
        fi
    done
    
    # Check for orphaned records
    ORPHANED=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c \
        "SELECT COUNT(*) FROM sprint_management.work_items WHERE parent_id IS NOT NULL AND parent_id NOT IN (SELECT id FROM sprint_management.work_items);")
    
    if [ "$ORPHANED" -eq 0 ]; then
        check_pass "No orphaned work items"
    else
        check_warn "Found $ORPHANED orphaned work items"
    fi
    
    # Check indexes
    INDEX_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c \
        "SELECT COUNT(*) FROM pg_indexes WHERE schemaname IN ('sprint_management', 'auth');")
    
    if [ "$INDEX_COUNT" -gt 10 ]; then
        check_pass "Database indexes present ($INDEX_COUNT indexes)"
    else
        check_warn "Low index count ($INDEX_COUNT indexes)"
    fi
}

# Configuration checks
check_configuration() {
    log_section "Configuration Verification"
    
    # Check required files
    FILES=(
        "docker-compose.yml"
        "Makefile"
        "go.mod"
        "Dockerfile"
    )
    
    for file in "${FILES[@]}"; do
        if [ -f "$file" ]; then
            check_pass "Configuration file exists: $file"
        else
            check_fail "Configuration file missing: $file"
        fi
    done
    
    # Check migrations directory
    if [ -d "migrations" ]; then
        MIGRATION_COUNT=$(find migrations -name "*.sql" | wc -l)
        if [ "$MIGRATION_COUNT" -gt 0 ]; then
            check_pass "Migration files present ($MIGRATION_COUNT files)"
        else
            check_warn "No migration files found"
        fi
    else
        check_fail "Migrations directory missing"
    fi
    
    # Check API specifications
    if [ -d "api" ]; then
        API_SPEC_COUNT=$(find api -name "*.yaml" -o -name "*.yml" | wc -l)
        if [ "$API_SPEC_COUNT" -gt 0 ]; then
            check_pass "API specifications present ($API_SPEC_COUNT files)"
        else
            check_warn "No API specification files found"
        fi
    else
        check_fail "API directory missing"
    fi
}

# Service checks
check_services() {
    log_section "Service Verification"
    
    # Check if services are running (Docker)
    if command -v docker &> /dev/null; then
        RUNNING_CONTAINERS=$(docker ps --format "{{.Names}}" | wc -l)
        if [ "$RUNNING_CONTAINERS" -gt 0 ]; then
            check_pass "Docker containers running ($RUNNING_CONTAINERS containers)"
        else
            check_warn "No Docker containers running"
        fi
    else
        check_warn "Docker not available, skipping container checks"
    fi
    
    # Check API health endpoint
    if command -v curl &> /dev/null; then
        if curl -s -f "$API_URL/health/live" > /dev/null 2>&1; then
            check_pass "API liveness check passed"
        else
            check_warn "API liveness check failed (service may not be running)"
        fi
        
        if curl -s -f "$API_URL/health/ready" > /dev/null 2>&1; then
            check_pass "API readiness check passed"
        else
            check_warn "API readiness check failed (service may not be ready)"
        fi
    else
        check_warn "curl not available, skipping API checks"
    fi
}

# NATS checks
check_nats() {
    log_section "NATS Verification"
    
    # Check NATS documentation
    if [ -f "docs/nats.md" ]; then
        check_pass "NATS documentation exists"
    else
        check_warn "NATS documentation missing"
    fi
    
    # Check NATS deployment config
    if [ -f "../deploy/nats/docker-compose.yml" ] || [ -f "../deploy/nats/k8s/deployment.yml" ]; then
        check_pass "NATS deployment configuration exists"
    else
        check_warn "NATS deployment configuration missing"
    fi
}

# Documentation checks
check_documentation() {
    log_section "Documentation Verification"
    
    DOCS=(
        "README.md"
        "docs/database-setup.md"
        "docs/nats.md"
        "docs/disaster-recovery.md"
    )
    
    for doc in "${DOCS[@]}"; do
        if [ -f "$doc" ]; then
            check_pass "Documentation exists: $doc"
        else
            check_warn "Documentation missing: $doc"
        fi
    done
}

# Data integrity checks
check_data_integrity() {
    log_section "Data Integrity Verification"
    
    # Check for duplicate IDs
    DUPLICATE_WORK_ITEMS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c \
        "SELECT COUNT(*) FROM (SELECT id, COUNT(*) FROM sprint_management.work_items GROUP BY id HAVING COUNT(*) > 1) AS duplicates;")
    
    if [ "$DUPLICATE_WORK_ITEMS" -eq 0 ]; then
        check_pass "No duplicate work item IDs"
    else
        check_fail "Found $DUPLICATE_WORK_ITEMS duplicate work item IDs"
    fi
    
    # Check timestamp consistency
    INVALID_TIMESTAMPS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c \
        "SELECT COUNT(*) FROM sprint_management.work_items WHERE created_at > updated_at;")
    
    if [ "$INVALID_TIMESTAMPS" -eq 0 ]; then
        check_pass "Timestamp consistency verified"
    else
        check_warn "Found $INVALID_TIMESTAMPS records with invalid timestamps"
    fi
    
    # Check sprint date validity
    INVALID_SPRINTS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c \
        "SELECT COUNT(*) FROM sprint_management.sprints WHERE end_date <= start_date;")
    
    if [ "$INVALID_SPRINTS" -eq 0 ]; then
        check_pass "Sprint date validation passed"
    else
        check_fail "Found $INVALID_SPRINTS sprints with invalid dates"
    fi
}

# Generate report
generate_report() {
    log_section "Verification Summary"
    
    TOTAL=$((PASSED + FAILED + WARNINGS))
    
    echo ""
    echo "Total Checks: $TOTAL"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo -e "${YELLOW}Warnings: $WARNINGS${NC}"
    echo ""
    
    if [ "$FAILED" -eq 0 ]; then
        if [ "$WARNINGS" -eq 0 ]; then
            echo -e "${GREEN}✓ System verification PASSED - All checks successful${NC}"
            return 0
        else
            echo -e "${YELLOW}⚠ System verification PASSED with warnings${NC}"
            return 0
        fi
    else
        echo -e "${RED}✗ System verification FAILED - $FAILED critical issues found${NC}"
        return 1
    fi
}

# Main execution
main() {
    echo "Sprint Management System - Verification Script"
    echo "=============================================="
    echo ""
    
    check_database
    check_configuration
    check_services
    check_nats
    check_documentation
    check_data_integrity
    
    generate_report
}

# Run main
main
