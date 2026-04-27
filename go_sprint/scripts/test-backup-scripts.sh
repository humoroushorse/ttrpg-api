#!/bin/bash
# Test script for backup and restore scripts
# Validates that backup scripts work correctly

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
PASSED=0
FAILED=0

log_info() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_fail() {
    echo -e "${RED}[✗]${NC} $1"
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

# Test 1: Check if backup scripts exist
test_scripts_exist() {
    log_section "Test 1: Checking if backup scripts exist"
    
    if [ -f "scripts/backup.sh" ]; then
        check_pass "backup.sh exists"
    else
        check_fail "backup.sh not found"
    fi
    
    if [ -f "scripts/backup-system.sh" ]; then
        check_pass "backup-system.sh exists"
    else
        check_fail "backup-system.sh not found"
    fi
    
    if [ -f "scripts/restore.sh" ]; then
        check_pass "restore.sh exists"
    else
        check_fail "restore.sh not found"
    fi
    
    if [ -f "scripts/verify-system.sh" ]; then
        check_pass "verify-system.sh exists"
    else
        check_fail "verify-system.sh not found"
    fi
}

# Test 2: Check if scripts are executable
test_scripts_executable() {
    log_section "Test 2: Checking if scripts are executable"
    
    if [ -x "scripts/backup.sh" ]; then
        check_pass "backup.sh is executable"
    else
        check_fail "backup.sh is not executable"
    fi
    
    if [ -x "scripts/backup-system.sh" ]; then
        check_pass "backup-system.sh is executable"
    else
        check_fail "backup-system.sh is not executable"
    fi
    
    if [ -x "scripts/restore.sh" ]; then
        check_pass "restore.sh is executable"
    else
        check_fail "restore.sh is not executable"
    fi
    
    if [ -x "scripts/verify-system.sh" ]; then
        check_pass "verify-system.sh is executable"
    else
        check_fail "verify-system.sh is not executable"
    fi
}

# Test 3: Validate script syntax
test_script_syntax() {
    log_section "Test 3: Validating script syntax"
    
    if bash -n scripts/backup.sh 2>/dev/null; then
        check_pass "backup.sh syntax is valid"
    else
        check_fail "backup.sh has syntax errors"
    fi
    
    if bash -n scripts/backup-system.sh 2>/dev/null; then
        check_pass "backup-system.sh syntax is valid"
    else
        check_fail "backup-system.sh has syntax errors"
    fi
    
    if bash -n scripts/restore.sh 2>/dev/null; then
        check_pass "restore.sh syntax is valid"
    else
        check_fail "restore.sh has syntax errors"
    fi
    
    if bash -n scripts/verify-system.sh 2>/dev/null; then
        check_pass "verify-system.sh syntax is valid"
    else
        check_fail "verify-system.sh has syntax errors"
    fi
}

# Test 4: Check documentation exists
test_documentation() {
    log_section "Test 4: Checking documentation"
    
    if [ -f "docs/backup-restore.md" ]; then
        check_pass "backup-restore.md exists"
    else
        check_fail "backup-restore.md not found"
    fi
    
    if [ -f "docs/disaster-recovery.md" ]; then
        check_pass "disaster-recovery.md exists"
    else
        check_fail "disaster-recovery.md not found"
    fi
}

# Test 5: Check Makefile targets
test_makefile_targets() {
    log_section "Test 5: Checking Makefile targets"
    
    if grep -q "^backup:" Makefile; then
        check_pass "Makefile has 'backup' target"
    else
        check_fail "Makefile missing 'backup' target"
    fi
    
    if grep -q "^backup-system:" Makefile; then
        check_pass "Makefile has 'backup-system' target"
    else
        check_fail "Makefile missing 'backup-system' target"
    fi
    
    if grep -q "^restore:" Makefile; then
        check_pass "Makefile has 'restore' target"
    else
        check_fail "Makefile missing 'restore' target"
    fi
    
    if grep -q "^verify-system:" Makefile; then
        check_pass "Makefile has 'verify-system' target"
    else
        check_fail "Makefile missing 'verify-system' target"
    fi
}

# Test 6: Check backup package exists
test_backup_package() {
    log_section "Test 6: Checking backup package"
    
    if [ -f "pkg/backup/scheduler.go" ]; then
        check_pass "backup scheduler implementation exists"
    else
        check_fail "backup scheduler implementation not found"
    fi
    
    if [ -f "pkg/backup/scheduler_test.go" ]; then
        check_pass "backup scheduler tests exist"
    else
        check_fail "backup scheduler tests not found"
    fi
    
    if [ -f "pkg/backup/integration_test.go" ]; then
        check_pass "backup integration tests exist"
    else
        check_fail "backup integration tests not found"
    fi
}

# Test 7: Validate backup directory structure
test_backup_structure() {
    log_section "Test 7: Validating backup directory structure"
    
    # Create test backup directory
    TEST_BACKUP_DIR="./test_backup_structure"
    mkdir -p "$TEST_BACKUP_DIR"
    
    # Test if we can create backup subdirectories
    if mkdir -p "$TEST_BACKUP_DIR"/{database,config,migrations,api,docs,nats}; then
        check_pass "Can create backup directory structure"
    else
        check_fail "Cannot create backup directory structure"
    fi
    
    # Cleanup
    rm -rf "$TEST_BACKUP_DIR"
}

# Test 8: Check required tools
test_required_tools() {
    log_section "Test 8: Checking required tools (optional)"
    
    if command -v pg_dump &> /dev/null; then
        check_pass "pg_dump is available"
    else
        echo -e "${YELLOW}[!]${NC} pg_dump not available (required for actual backups)"
    fi
    
    if command -v psql &> /dev/null; then
        check_pass "psql is available"
    else
        echo -e "${YELLOW}[!]${NC} psql not available (required for restore)"
    fi
    
    if command -v sha256sum &> /dev/null || command -v shasum &> /dev/null; then
        check_pass "checksum tool is available"
    else
        echo -e "${YELLOW}[!]${NC} checksum tool not available"
    fi
}

# Generate report
generate_report() {
    log_section "Test Summary"
    
    TOTAL=$((PASSED + FAILED))
    
    echo ""
    echo "Total Tests: $TOTAL"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""
    
    if [ "$FAILED" -eq 0 ]; then
        echo -e "${GREEN}✓ All tests PASSED${NC}"
        return 0
    else
        echo -e "${RED}✗ Some tests FAILED${NC}"
        return 1
    fi
}

# Main execution
main() {
    echo "Backup Scripts Test Suite"
    echo "========================="
    echo ""
    
    test_scripts_exist
    test_scripts_executable
    test_script_syntax
    test_documentation
    test_makefile_targets
    test_backup_package
    test_backup_structure
    test_required_tools
    
    generate_report
}

# Run tests
main
