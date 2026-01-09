#!/bin/bash

# Sprint Management System - Kubernetes Deployment Script
# This script deploys the Sprint Management System to a Kubernetes cluster

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="sprint-management"
TIMEOUT="300s"

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if kubectl is installed
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed. Please install kubectl first."
        exit 1
    fi
    print_info "kubectl is installed"
}

# Function to check if cluster is accessible
check_cluster() {
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    print_info "Connected to Kubernetes cluster"
}

# Function to check secrets
check_secrets() {
    print_warn "Checking secrets configuration..."
    
    if grep -q "CHANGE_ME_IN_PRODUCTION" secrets.yaml; then
        print_error "Found placeholder values in secrets.yaml!"
        print_error "Please update secrets.yaml with actual values before deploying."
        print_error "Never commit actual secrets to version control!"
        exit 1
    fi
    
    print_info "Secrets configuration looks good"
}

# Function to wait for deployment
wait_for_deployment() {
    local deployment=$1
    print_info "Waiting for $deployment to be ready..."
    
    if kubectl wait --for=condition=available deployment/$deployment -n $NAMESPACE --timeout=$TIMEOUT; then
        print_info "$deployment is ready"
    else
        print_error "$deployment failed to become ready"
        kubectl get pods -n $NAMESPACE -l app=$deployment
        kubectl describe deployment/$deployment -n $NAMESPACE
        exit 1
    fi
}

# Function to wait for pods
wait_for_pods() {
    local label=$1
    print_info "Waiting for pods with label $label to be ready..."
    
    if kubectl wait --for=condition=ready pod -l $label -n $NAMESPACE --timeout=$TIMEOUT; then
        print_info "Pods with label $label are ready"
    else
        print_error "Pods with label $label failed to become ready"
        kubectl get pods -n $NAMESPACE -l $label
        exit 1
    fi
}

# Function to deploy component
deploy_component() {
    local component=$1
    local file=$2
    
    print_info "Deploying $component..."
    kubectl apply -f $file
}

# Main deployment function
deploy() {
    print_info "Starting Sprint Management System deployment..."
    
    # Check prerequisites
    check_kubectl
    check_cluster
    
    # Check if we should skip secrets check (for CI/CD)
    if [ "$SKIP_SECRETS_CHECK" != "true" ]; then
        check_secrets
    fi
    
    # Create namespace
    print_info "Creating namespace..."
    kubectl apply -f namespace.yaml
    
    # Apply secrets
    print_info "Applying secrets..."
    kubectl apply -f secrets.yaml
    
    # Apply configmaps
    print_info "Applying configmaps..."
    kubectl apply -f configmap.yaml
    
    # Deploy PostgreSQL
    deploy_component "PostgreSQL" "postgres-deployment.yaml"
    wait_for_pods "app=postgres"
    
    # Deploy NATS
    deploy_component "NATS" "nats-deployment.yaml"
    wait_for_pods "app=nats"
    
    # Deploy Keycloak
    deploy_component "Keycloak" "keycloak-deployment.yaml"
    wait_for_pods "app=keycloak"
    
    # Deploy Auth Service
    deploy_component "Auth Service" "auth-service-deployment.yaml"
    wait_for_deployment "auth-service"
    
    # Deploy Sprint Service
    deploy_component "Sprint Service" "sprint-service-deployment.yaml"
    wait_for_deployment "sprint-service"
    
    # Apply network policies
    print_info "Applying network policies..."
    kubectl apply -f network-policy.yaml
    
    # Apply ingress
    print_info "Applying ingress..."
    kubectl apply -f ingress.yaml
    
    # Display status
    print_info "Deployment complete!"
    echo ""
    print_info "Checking deployment status..."
    kubectl get all -n $NAMESPACE
    echo ""
    print_info "To access the services:"
    echo "  Sprint Service: kubectl port-forward -n $NAMESPACE service/sprint-service 8080:8080"
    echo "  Auth Service: kubectl port-forward -n $NAMESPACE service/auth-service 8081:8080"
    echo "  Keycloak: kubectl port-forward -n $NAMESPACE service/keycloak-service 8082:8080"
    echo ""
    print_info "To view logs:"
    echo "  Sprint Service: kubectl logs -f -n $NAMESPACE deployment/sprint-service"
    echo "  Auth Service: kubectl logs -f -n $NAMESPACE deployment/auth-service"
}

# Function to undeploy
undeploy() {
    print_warn "Undeploying Sprint Management System..."
    
    read -p "Are you sure you want to delete all resources? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        print_info "Undeploy cancelled"
        exit 0
    fi
    
    print_info "Deleting namespace and all resources..."
    kubectl delete namespace $NAMESPACE
    
    print_info "Undeploy complete"
}

# Function to show status
status() {
    print_info "Sprint Management System Status"
    echo ""
    
    print_info "Pods:"
    kubectl get pods -n $NAMESPACE
    echo ""
    
    print_info "Services:"
    kubectl get services -n $NAMESPACE
    echo ""
    
    print_info "Deployments:"
    kubectl get deployments -n $NAMESPACE
    echo ""
    
    print_info "HPA:"
    kubectl get hpa -n $NAMESPACE
    echo ""
    
    print_info "Ingress:"
    kubectl get ingress -n $NAMESPACE
}

# Function to show logs
logs() {
    local service=$1
    
    if [ -z "$service" ]; then
        print_error "Please specify a service: sprint-service, auth-service, postgres, nats, keycloak"
        exit 1
    fi
    
    print_info "Showing logs for $service..."
    kubectl logs -f -n $NAMESPACE deployment/$service
}

# Function to restart service
restart() {
    local service=$1
    
    if [ -z "$service" ]; then
        print_error "Please specify a service: sprint-service, auth-service, keycloak"
        exit 1
    fi
    
    print_info "Restarting $service..."
    kubectl rollout restart deployment/$service -n $NAMESPACE
    kubectl rollout status deployment/$service -n $NAMESPACE
}

# Function to scale service
scale() {
    local service=$1
    local replicas=$2
    
    if [ -z "$service" ] || [ -z "$replicas" ]; then
        print_error "Usage: $0 scale <service> <replicas>"
        exit 1
    fi
    
    print_info "Scaling $service to $replicas replicas..."
    kubectl scale deployment/$service --replicas=$replicas -n $NAMESPACE
    kubectl rollout status deployment/$service -n $NAMESPACE
}

# Function to show help
show_help() {
    cat << EOF
Sprint Management System - Kubernetes Deployment Script

Usage: $0 [command]

Commands:
  deploy      Deploy the Sprint Management System
  undeploy    Remove all resources
  status      Show deployment status
  logs        Show logs for a service
              Usage: $0 logs <service>
  restart     Restart a service
              Usage: $0 restart <service>
  scale       Scale a service
              Usage: $0 scale <service> <replicas>
  help        Show this help message

Services:
  sprint-service    Main sprint management service
  auth-service      Authentication service
  postgres          PostgreSQL database
  nats              NATS message broker
  keycloak          Keycloak identity provider

Examples:
  $0 deploy
  $0 status
  $0 logs sprint-service
  $0 restart sprint-service
  $0 scale sprint-service 5
  $0 undeploy

Environment Variables:
  SKIP_SECRETS_CHECK    Skip secrets validation (for CI/CD)
                        Usage: SKIP_SECRETS_CHECK=true $0 deploy

EOF
}

# Main script
case "$1" in
    deploy)
        deploy
        ;;
    undeploy)
        undeploy
        ;;
    status)
        status
        ;;
    logs)
        logs "$2"
        ;;
    restart)
        restart "$2"
        ;;
    scale)
        scale "$2" "$3"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
