# Kubernetes Deployment Guide

This guide provides comprehensive instructions for deploying the Sprint Management System to a Kubernetes cluster.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Architecture](#architecture)
- [Deployment Steps](#deployment-steps)
- [Configuration](#configuration)
- [Health Checks](#health-checks)
- [Scaling](#scaling)
- [Monitoring](#monitoring)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Production Considerations](#production-considerations)

## Overview

The Sprint Management System is deployed as a microservices architecture on Kubernetes with the following components:

- **Sprint Service**: Main application (3-20 replicas with auto-scaling)
- **Auth Service**: Authentication service (2-10 replicas with auto-scaling)
- **PostgreSQL**: Database with persistent storage
- **NATS**: Message broker with JetStream
- **Keycloak**: Identity and access management

## Prerequisites

### Required Tools

- **Kubernetes Cluster**: v1.24 or higher
- **kubectl**: Configured to access your cluster
- **Helm**: v3.0+ (for installing dependencies)

### Optional Tools

- **k9s**: Terminal UI for Kubernetes
- **kubectx/kubens**: Context and namespace switching
- **stern**: Multi-pod log tailing

### Cluster Requirements

- **Nodes**: Minimum 3 nodes for high availability
- **CPU**: Minimum 8 cores total
- **Memory**: Minimum 16GB total
- **Storage**: Support for PersistentVolumes (15GB minimum)
- **Ingress Controller**: nginx-ingress or similar
- **Cert Manager**: For TLS certificate management

## Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────┐
│                    Ingress Controller                   │
│              (nginx-ingress + cert-manager)             │
└─────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│Sprint Service│    │ Auth Service │    │  Keycloak    │
│  (3-20 pods) │    │  (2-10 pods) │    │   (1 pod)    │
└──────────────┘    └──────────────┘    └──────────────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  PostgreSQL  │    │     NATS     │    │  Prometheus  │
│   (1 pod)    │    │   (1 pod)    │    │  (optional)  │
│  + PV (10GB) │    │  + PV (5GB)  │    │              │
└──────────────┘    └──────────────┘    └──────────────┘
```

### Network Policies

Network policies restrict traffic between components:

- Sprint Service → PostgreSQL, NATS, Auth Service, Keycloak
- Auth Service → PostgreSQL, NATS, Keycloak
- PostgreSQL ← Sprint Service, Auth Service, Keycloak only
- NATS ← Sprint Service, Auth Service only

## Deployment Steps

### Step 1: Prepare Secrets

**CRITICAL**: Never commit actual secrets to version control!

1. Generate secure passwords:

```bash
# Generate passwords
export POSTGRES_PASSWORD=$(openssl rand -base64 32)
export KEYCLOAK_ADMIN_PASSWORD=$(openssl rand -base64 32)
export KEYCLOAK_CLIENT_SECRET=$(openssl rand -base64 32)
export JWT_SECRET=$(openssl rand -base64 64)

# Display for verification (save these securely!)
echo "POSTGRES_PASSWORD: $POSTGRES_PASSWORD"
echo "KEYCLOAK_ADMIN_PASSWORD: $KEYCLOAK_ADMIN_PASSWORD"
echo "KEYCLOAK_CLIENT_SECRET: $KEYCLOAK_CLIENT_SECRET"
echo "JWT_SECRET: $JWT_SECRET"
```

2. Update `k8s/secrets.yaml` with generated values

3. For production, use a secrets management solution:
   - [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
   - [External Secrets Operator](https://external-secrets.io/)
   - [HashiCorp Vault](https://www.vaultproject.io/)
   - Cloud provider secrets (AWS Secrets Manager, GCP Secret Manager, Azure Key Vault)

### Step 2: Configure Domain Names

Update `k8s/ingress.yaml` with your domain names:

```yaml
spec:
  tls:
  - hosts:
    - api.yourdomain.com        # Replace with your domain
    - auth.yourdomain.com       # Replace with your domain
    - keycloak.yourdomain.com   # Replace with your domain
```

Update `k8s/configmap.yaml` with CORS settings:

```yaml
data:
  CORS_ALLOWED_ORIGINS: "https://yourdomain.com"  # Replace with your domain
```

### Step 3: Install Prerequisites

#### Install Ingress Controller

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer
```

#### Install Cert Manager

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true
```

#### Create ClusterIssuer for Let's Encrypt

```bash
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com  # Replace with your email
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF
```

### Step 4: Deploy Using Script

The easiest way to deploy is using the provided script:

```bash
cd go_sprint/k8s

# Deploy everything
./deploy.sh deploy

# Check status
./deploy.sh status

# View logs
./deploy.sh logs sprint-service
```

### Step 5: Manual Deployment (Alternative)

If you prefer manual deployment:

```bash
cd go_sprint/k8s

# 1. Create namespace
kubectl apply -f namespace.yaml

# 2. Apply secrets and configmaps
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml

# 3. Deploy infrastructure
kubectl apply -f postgres-deployment.yaml
kubectl wait --for=condition=ready pod -l app=postgres -n sprint-management --timeout=300s

kubectl apply -f nats-deployment.yaml
kubectl wait --for=condition=ready pod -l app=nats -n sprint-management --timeout=300s

kubectl apply -f keycloak-deployment.yaml
kubectl wait --for=condition=ready pod -l app=keycloak -n sprint-management --timeout=300s

# 4. Deploy services
kubectl apply -f auth-service-deployment.yaml
kubectl wait --for=condition=available deployment/auth-service -n sprint-management --timeout=300s

kubectl apply -f sprint-service-deployment.yaml
kubectl wait --for=condition=available deployment/sprint-service -n sprint-management --timeout=300s

# 5. Apply network policies
kubectl apply -f network-policy.yaml

# 6. Apply ingress
kubectl apply -f ingress.yaml

# 7. Apply pod disruption budgets (optional but recommended)
kubectl apply -f pod-disruption-budget.yaml

# 8. Apply service monitors (if using Prometheus Operator)
kubectl apply -f service-monitor.yaml
```

### Step 6: Verify Deployment

```bash
# Check all resources
kubectl get all -n sprint-management

# Check pod status
kubectl get pods -n sprint-management

# Check services
kubectl get services -n sprint-management

# Check ingress
kubectl get ingress -n sprint-management

# Check HPA
kubectl get hpa -n sprint-management
```

### Step 7: Run Database Migrations

```bash
# Port-forward to PostgreSQL
kubectl port-forward -n sprint-management service/postgres-service 5432:5432 &

# Run migrations
cd go_sprint
export DATABASE_URL="postgresql://postgres:${POSTGRES_PASSWORD}@localhost:5432/sprint_management?sslmode=disable"
make migrate-up

# Stop port-forward
kill %1
```

## Configuration

### Environment Variables

Configuration is managed through ConfigMaps and Secrets:

#### ConfigMap: sprint-config

Non-sensitive configuration:
- Logging settings (level, format)
- Server settings (port, timeouts)
- NATS connection settings
- Database connection pool settings
- Cache settings
- Rate limiting settings
- CORS settings

#### Secret: database-secrets

Database credentials:
- PostgreSQL username and password
- Database connection URLs (master and replica)

#### Secret: keycloak-secrets

Keycloak credentials:
- Admin username and password
- Client ID and secret

#### Secret: jwt-secrets

JWT signing secrets

### Updating Configuration

To update non-sensitive configuration:

```bash
# Edit ConfigMap
kubectl edit configmap sprint-config -n sprint-management

# Restart services to pick up changes
kubectl rollout restart deployment/sprint-service -n sprint-management
kubectl rollout restart deployment/auth-service -n sprint-management
```

To update secrets:

```bash
# Edit Secret
kubectl edit secret database-secrets -n sprint-management

# Restart services
kubectl rollout restart deployment/sprint-service -n sprint-management
kubectl rollout restart deployment/auth-service -n sprint-management
```

## Health Checks

### Liveness Probes

Liveness probes check if the application is running:

- **Sprint Service**: `GET /health/live` (port 8080)
- **Auth Service**: `GET /health/live` (port 8080)
- **PostgreSQL**: `pg_isready` command
- **NATS**: `GET /healthz` (port 8222)
- **Keycloak**: `GET /health/live` (port 8080)

### Readiness Probes

Readiness probes check if the application is ready to serve traffic:

- **Sprint Service**: `GET /health/ready` (port 8080)
- **Auth Service**: `GET /health/ready` (port 8080)
- **PostgreSQL**: `pg_isready` command
- **NATS**: `GET /healthz` (port 8222)
- **Keycloak**: `GET /health/ready` (port 8080)

### Manual Health Checks

```bash
# Sprint Service
kubectl exec -it -n sprint-management deployment/sprint-service -- curl http://localhost:8080/health/live
kubectl exec -it -n sprint-management deployment/sprint-service -- curl http://localhost:8080/health/ready

# Auth Service
kubectl exec -it -n sprint-management deployment/auth-service -- curl http://localhost:8080/health/live
kubectl exec -it -n sprint-management deployment/auth-service -- curl http://localhost:8080/health/ready
```

## Scaling

### Horizontal Pod Autoscaling

HPA is configured for both services:

**Sprint Service**:
- Min replicas: 3
- Max replicas: 20
- Target CPU: 70%
- Target Memory: 80%

**Auth Service**:
- Min replicas: 2
- Max replicas: 10
- Target CPU: 70%
- Target Memory: 80%

### Manual Scaling

```bash
# Scale Sprint Service
kubectl scale deployment/sprint-service --replicas=5 -n sprint-management

# Scale Auth Service
kubectl scale deployment/auth-service --replicas=3 -n sprint-management

# Using the deploy script
./deploy.sh scale sprint-service 5
```

### View HPA Status

```bash
# List all HPAs
kubectl get hpa -n sprint-management

# Detailed HPA information
kubectl describe hpa sprint-service-hpa -n sprint-management

# Watch HPA in real-time
kubectl get hpa -n sprint-management --watch
```

## Monitoring

### Prometheus Metrics

All services expose Prometheus metrics at `/metrics`:

```bash
# Port-forward to access metrics
kubectl port-forward -n sprint-management deployment/sprint-service 8080:8080

# Access metrics
curl http://localhost:8080/metrics
```

### Service Monitors

If using Prometheus Operator, ServiceMonitors are configured:

```bash
kubectl get servicemonitor -n sprint-management
```

### Logs

View logs for services:

```bash
# Sprint Service logs
kubectl logs -f -n sprint-management deployment/sprint-service

# Auth Service logs
kubectl logs -f -n sprint-management deployment/auth-service

# All pods with label
kubectl logs -f -n sprint-management -l app=sprint-service

# Using stern (multi-pod log tailing)
stern -n sprint-management sprint-service

# Using the deploy script
./deploy.sh logs sprint-service
```

### Events

View cluster events:

```bash
# Recent events
kubectl get events -n sprint-management --sort-by='.lastTimestamp'

# Watch events in real-time
kubectl get events -n sprint-management --watch
```

## Security

### Pod Security

All application pods run with security best practices:

- **Non-root user**: UID 1000
- **Read-only root filesystem**: Prevents file modifications
- **Dropped capabilities**: All Linux capabilities dropped
- **No privilege escalation**: Prevents gaining additional privileges

### Network Policies

Network policies restrict traffic between components. Only necessary connections are allowed.

### Secrets Management

**Production Recommendations**:

1. **Use External Secrets**: Integrate with Vault, AWS Secrets Manager, etc.
2. **Enable Encryption at Rest**: Encrypt Kubernetes secrets
3. **Rotate Secrets Regularly**: Implement secret rotation
4. **Use RBAC**: Restrict access to secrets
5. **Audit Access**: Monitor secret access

### TLS/SSL

- **Ingress TLS**: Managed by cert-manager with Let's Encrypt
- **Internal TLS**: Consider using service mesh (Istio, Linkerd) for mTLS

### RBAC

Create service accounts with minimal permissions:

```bash
kubectl create serviceaccount sprint-service -n sprint-management
kubectl create serviceaccount auth-service -n sprint-management
```

## Troubleshooting

### Common Issues

#### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n sprint-management

# Describe pod for events
kubectl describe pod <pod-name> -n sprint-management

# Check logs
kubectl logs <pod-name> -n sprint-management

# Check previous logs if pod restarted
kubectl logs <pod-name> -n sprint-management --previous
```

#### Database Connection Issues

```bash
# Test PostgreSQL connectivity from pod
kubectl exec -it -n sprint-management deployment/sprint-service -- sh
nc -zv postgres-service 5432

# Check PostgreSQL logs
kubectl logs -n sprint-management deployment/postgres

# Verify secret values
kubectl get secret database-secrets -n sprint-management -o yaml
```

#### NATS Connection Issues

```bash
# Check NATS status
kubectl exec -it -n sprint-management deployment/nats -- nats-server --signal status

# Test NATS connectivity
kubectl exec -it -n sprint-management deployment/sprint-service -- sh
nc -zv nats-service 4222

# Check NATS logs
kubectl logs -n sprint-management deployment/nats
```

#### Service Not Accessible via Ingress

```bash
# Check ingress status
kubectl get ingress -n sprint-management
kubectl describe ingress sprint-management-ingress -n sprint-management

# Check ingress controller logs
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# Check service endpoints
kubectl get endpoints -n sprint-management

# Test service directly
kubectl port-forward -n sprint-management service/sprint-service 8080:8080
curl http://localhost:8080/health/live
```

#### High Memory/CPU Usage

```bash
# Check resource usage
kubectl top pods -n sprint-management
kubectl top nodes

# Check HPA status
kubectl get hpa -n sprint-management

# Describe pod for resource limits
kubectl describe pod <pod-name> -n sprint-management
```

### Debug Mode

Enable debug logging:

```bash
# Set debug log level
kubectl set env deployment/sprint-service LOG_LEVEL=debug -n sprint-management
kubectl set env deployment/auth-service LOG_LEVEL=debug -n sprint-management

# Revert to info level
kubectl set env deployment/sprint-service LOG_LEVEL=info -n sprint-management
kubectl set env deployment/auth-service LOG_LEVEL=info -n sprint-management
```

## Production Considerations

### High Availability

1. **Multiple Replicas**: Run at least 3 replicas of each service
2. **Pod Disruption Budgets**: Ensure minimum availability during updates
3. **Anti-Affinity**: Spread pods across nodes and availability zones
4. **Database Replication**: Use PostgreSQL streaming replication
5. **NATS Clustering**: Deploy NATS in cluster mode with 3+ nodes

### Performance Optimization

1. **Resource Tuning**: Adjust CPU/memory based on actual usage
2. **Connection Pooling**: Optimize database connection pool settings
3. **Caching**: Enable and tune caching settings
4. **Database Indexing**: Ensure proper indexes are in place
5. **CDN**: Use CDN for static assets

### Monitoring and Alerting

1. **Prometheus**: Collect metrics from all services
2. **Grafana**: Create dashboards for visualization
3. **Alertmanager**: Configure alerts for critical issues
4. **Distributed Tracing**: Use Jaeger or Zipkin
5. **Log Aggregation**: Use ELK stack or similar

### Backup and Disaster Recovery

1. **Database Backups**: Automated daily backups with retention
2. **Cluster Backups**: Use Velero for full cluster backups
3. **Disaster Recovery Plan**: Document and test recovery procedures
4. **Multi-Region**: Consider multi-region deployment for critical systems

### Cost Optimization

1. **Right-sizing**: Adjust resource requests/limits based on actual usage
2. **Spot Instances**: Use spot/preemptible instances for non-critical workloads
3. **Auto-scaling**: Configure HPA to scale down during low traffic
4. **Storage Optimization**: Use appropriate storage classes

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Kubernetes Best Practices](https://kubernetes.io/docs/concepts/configuration/overview/)
- [NATS on Kubernetes](https://docs.nats.io/running-a-nats-service/nats-kubernetes)
- [PostgreSQL on Kubernetes](https://www.postgresql.org/docs/current/high-availability.html)
- [Keycloak on Kubernetes](https://www.keycloak.org/server/containers)
- [Prometheus Operator](https://prometheus-operator.dev/)

## Support

For issues and questions:
- Review this documentation
- Check the troubleshooting section
- Review application logs and Kubernetes events
- Check the main project README
