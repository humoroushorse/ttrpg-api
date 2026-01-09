#!/bin/bash

# Kubernetes Manifests Validation Script
# This script validates all Kubernetes manifests for syntax and best practices

# Don't exit on error - we want to collect all validation results
set +e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNING_CHECKS=0

# Cluster availability flag
CLUSTER_AVAILABLE=false

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED_CHECKS++))
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    ((WARNING_CHECKS++))
}

print_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED_CHECKS++))
}

# Function to check if kubectl is installed
check_kubectl() {
    ((TOTAL_CHECKS++))
    if command -v kubectl &> /dev/null; then
        print_success "kubectl is installed"
        
        # Check if cluster is accessible (don't exit on failure)
        if kubectl cluster-info &> /dev/null 2>&1; then
            CLUSTER_AVAILABLE=true
            print_success "Kubernetes cluster is accessible"
        else
            CLUSTER_AVAILABLE=false
            print_warn "Kubernetes cluster not accessible - skipping cluster validation"
        fi
        return 0
    else
        print_warn "kubectl is not installed - skipping cluster validation"
        CLUSTER_AVAILABLE=false
        return 0
    fi
}

# Function to validate YAML syntax
validate_yaml_syntax() {
    local file=$1
    ((TOTAL_CHECKS++))
    
    # If cluster is available, use kubectl validation
    if [ "$CLUSTER_AVAILABLE" = true ]; then
        if kubectl apply --dry-run=client -f "$file" &> /dev/null; then
            print_success "Valid YAML syntax: $file"
            return 0
        else
            print_error "Invalid YAML syntax: $file"
            kubectl apply --dry-run=client -f "$file" 2>&1 | head -5
            return 1
        fi
    else
        # Basic YAML syntax check using Python (if available)
        if command -v python3 &> /dev/null && python3 -c "import yaml" 2> /dev/null; then
            if python3 -c "import yaml; list(yaml.safe_load_all(open('$file')))" 2> /dev/null; then
                print_success "Valid YAML syntax: $file"
                return 0
            else
                print_error "Invalid YAML syntax: $file"
                return 1
            fi
        else
            # Just check if file is readable and not empty
            if [ -r "$file" ] && [ -s "$file" ]; then
                print_success "File is readable and not empty: $file"
                return 0
            else
                print_error "Cannot read file or file is empty: $file"
                return 1
            fi
        fi
    fi
}

# Function to check for health checks
check_health_probes() {
    local file=$1
    local deployment_name=$2
    ((TOTAL_CHECKS++))
    
    if grep -q "livenessProbe" "$file" && grep -q "readinessProbe" "$file"; then
        print_success "Health probes configured: $deployment_name"
        return 0
    else
        print_error "Missing health probes: $deployment_name"
        return 1
    fi
}

# Function to check for resource limits
check_resource_limits() {
    local file=$1
    local deployment_name=$2
    ((TOTAL_CHECKS++))
    
    if grep -q "resources:" "$file" && grep -q "limits:" "$file" && grep -q "requests:" "$file"; then
        print_success "Resource limits configured: $deployment_name"
        return 0
    else
        print_error "Missing resource limits: $deployment_name"
        return 1
    fi
}

# Function to check for security context
check_security_context() {
    local file=$1
    local deployment_name=$2
    ((TOTAL_CHECKS++))
    
    if grep -q "securityContext:" "$file"; then
        if grep -q "runAsNonRoot: true" "$file" || grep -q "runAsUser:" "$file"; then
            print_success "Security context configured: $deployment_name"
            return 0
        else
            print_warn "Security context present but may need review: $deployment_name"
            return 0
        fi
    else
        print_warn "No security context found: $deployment_name"
        return 0
    fi
}

# Function to check for secrets placeholders
check_secrets_placeholders() {
    ((TOTAL_CHECKS++))
    
    if grep -q "CHANGE_ME_IN_PRODUCTION" secrets.yaml; then
        print_warn "Placeholder values found in secrets.yaml - update before deploying to production"
        return 0
    else
        print_success "No placeholder values in secrets.yaml"
        return 0
    fi
}

# Function to check for required files
check_required_files() {
    local required_files=(
        "namespace.yaml"
        "configmap.yaml"
        "secrets.yaml"
        "sprint-service-deployment.yaml"
        "auth-service-deployment.yaml"
        "postgres-deployment.yaml"
        "nats-deployment.yaml"
        "keycloak-deployment.yaml"
        "ingress.yaml"
        "network-policy.yaml"
    )
    
    for file in "${required_files[@]}"; do
        ((TOTAL_CHECKS++))
        if [ -f "$file" ]; then
            print_success "Required file exists: $file"
        else
            print_error "Missing required file: $file"
        fi
    done
}

# Main validation function
main() {
    print_info "Starting Kubernetes manifests validation..."
    echo ""
    
    # Check kubectl
    print_info "Checking prerequisites..."
    check_kubectl
    echo ""
    
    # Check required files
    print_info "Checking required files..."
    check_required_files
    echo ""
    
    # Validate YAML syntax
    print_info "Validating YAML syntax..."
    for file in *.yaml; do
        if [ -f "$file" ]; then
            validate_yaml_syntax "$file"
        fi
    done
    echo ""
    
    # Check deployment configurations
    print_info "Checking deployment configurations..."
    
    # Sprint Service
    if [ -f "sprint-service-deployment.yaml" ]; then
        check_health_probes "sprint-service-deployment.yaml" "sprint-service"
        check_resource_limits "sprint-service-deployment.yaml" "sprint-service"
        check_security_context "sprint-service-deployment.yaml" "sprint-service"
    fi
    
    # Auth Service
    if [ -f "auth-service-deployment.yaml" ]; then
        check_health_probes "auth-service-deployment.yaml" "auth-service"
        check_resource_limits "auth-service-deployment.yaml" "auth-service"
        check_security_context "auth-service-deployment.yaml" "auth-service"
    fi
    
    # PostgreSQL
    if [ -f "postgres-deployment.yaml" ]; then
        check_health_probes "postgres-deployment.yaml" "postgres"
        check_resource_limits "postgres-deployment.yaml" "postgres"
    fi
    
    # NATS
    if [ -f "nats-deployment.yaml" ]; then
        check_health_probes "nats-deployment.yaml" "nats"
        check_resource_limits "nats-deployment.yaml" "nats"
    fi
    
    # Keycloak
    if [ -f "keycloak-deployment.yaml" ]; then
        check_health_probes "keycloak-deployment.yaml" "keycloak"
        check_resource_limits "keycloak-deployment.yaml" "keycloak"
    fi
    
    echo ""
    
    # Check secrets
    print_info "Checking secrets configuration..."
    check_secrets_placeholders
    echo ""
    
    # Summary
    echo "================================"
    echo "Validation Summary"
    echo "================================"
    echo "Total checks: $TOTAL_CHECKS"
    echo -e "${GREEN}Passed: $PASSED_CHECKS${NC}"
    echo -e "${YELLOW}Warnings: $WARNING_CHECKS${NC}"
    echo -e "${RED}Failed: $FAILED_CHECKS${NC}"
    echo "================================"
    
    if [ $FAILED_CHECKS -eq 0 ]; then
        echo -e "${GREEN}All critical checks passed!${NC}"
        if [ $WARNING_CHECKS -gt 0 ]; then
            echo -e "${YELLOW}Please review warnings before production deployment.${NC}"
        fi
        exit 0
    else
        echo -e "${RED}Some checks failed. Please fix the issues before deploying.${NC}"
        exit 1
    fi
}

# Run main function
main
