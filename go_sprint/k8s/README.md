# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying the Sprint Management System to a Kubernetes cluster.

## Overview

The Sprint Management System consists of the following components:

- **Sprint Service**: Main application service for sprint and work item management
- **Auth Service**: Authentication and authorization service
- **PostgreSQL**: Primary database with master/replica support
- **NATS**: Message broker for inter-service communication
- **Keycloak**: Identity and access management

## Prerequisites

- Kubernetes cluster (v1.24+)
- kubectl configured to access your cluster
- Helm (optional, for cert-manager and ingress-nginx)
- Container registry access (GitHub Container Registry or similar)

## Quick Start

### 1. Create Namespace

```bash
kubectl apply -f namespace.yaml
```

### 2. Configure Secrets

**IMPORTANT**: Never commit actual secrets to version control!

Edit `secrets.yaml` and replace all `CHANGE_ME_IN_PRODUCTION` values:

```bash
# Generate secure passwords
export POSTGRES_PASSWORD=$(openssl rand -base64 32)
export KEYCLOAK_ADMIN_PASSWORD=$(openssl rand -base64 32)
export JWT_SECRET=$(openssl rand -base64 64)

# Update secrets.yaml with your values
kubectl apply -f secrets.yaml
```

For production, use a secrets management solution:
- [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
- [External Secrets Operator](https://external-secrets.io/)
- [HashiCorp Vault](https://www.vaultproject.io/)

### 3. Apply ConfigMaps

```bash
kubectl apply -f configmap.yaml
```

### 4. Deploy Infrastructure

Deploy in order to ensure dependencies are ready:

```bash
# PostgreSQL
kubectl apply -f postgres-deployment.yaml

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n sprint-management --timeout=300s

# NATS
kubectl apply -f nats-deployment.yaml

# Wait for NATS to be ready
kubectl wait --for=condition=ready pod -l app=nats -n sprint-management --timeout=300s

# Keycloak
kubectl apply -f keycloak-deployment.yaml

# Wait for Keycloak to be ready
kubectl wait --for=condition=ready pod -l app=keycloak -n sprint-management --timeout=300s
```

### 5. Deploy Services

```bash
# Auth Service
kubectl apply -f auth-service-deployment.yaml

# Wait for Auth Service to be ready
kubectl wait --for=condition=ready pod -l app=auth-service -n sprint-management --timeout=300s

# Sprint Service
kubectl apply -f sprint-service-deployment.yaml

# Wait for Sprint Service to be ready
kubectl wait --for=condition=ready pod -l app=sprint-service -n sprint-management --timeout=300s
```

### 6. Apply Network Policies

```bash
kubectl apply -f network-policy.yaml
```

### 7. Configure Ingress

First, install ingress-nginx if not already installed:

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace
```

Install cert-manager for TLS certificates:

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true
```

Update `ingress.yaml` with your domain names and apply:

```bash
kubectl apply -f ingress.yaml
```

## Component Details

### Sprint Service

- **Replicas**: 3 (auto-scales 3-20 based on CPU/memory)
- **Resources**: 
  - Requests: 512Mi memory, 500m CPU
  - Limits: 1Gi memory, 2000m CPU
- **Health Checks**: 
  - Liveness: `/health/live`
  - Readiness: `/health/ready`
- **Ports**: 8080 (HTTP)

### Auth Service

- **Replicas**: 2 (auto-scales 2-10 based on CPU/memory)
- **Resources**: 
  - Requests: 256Mi memory, 250m CPU
  - Limits: 512Mi memory, 1000m CPU
- **Health Checks**: 
  - Liveness: `/health/live`
  - Readiness: `/health/ready`
- **Ports**: 8080 (HTTP)

### PostgreSQL

- **Replicas**: 1 (StatefulSet recommended for production)
- **Storage**: 10Gi PersistentVolume
- **Resources**: 
  - Requests: 512Mi memory, 500m CPU
  - Limits: 2Gi memory, 2000m CPU
- **Health Checks**: `pg_isready`
- **Ports**: 5432

### NATS

- **Replicas**: 1 (cluster mode recommended for production)
- **Storage**: 5Gi PersistentVolume (for JetStream)
- **Resources**: 
  - Requests: 256Mi memory, 250m CPU
  - Limits: 512Mi memory, 1000m CPU
- **Health Checks**: `/healthz`
- **Ports**: 4222 (client), 8222 (monitoring), 6222 (cluster)

### Keycloak

- **Replicas**: 1 (2+ recommended for production)
- **Resources**: 
  - Requests: 512Mi memory, 500m CPU
  - Limits: 2Gi memory, 2000m CPU
- **Health Checks**: 
  - Liveness: `/health/live`
  - Readiness: `/health/ready`
- **Ports**: 8080 (HTTP)

## Configuration

### Environment Variables

All configuration is managed through ConfigMaps and Secrets:

- **ConfigMap**: `sprint-config` - Non-sensitive configuration
- **Secret**: `database-secrets` - Database credentials and URLs
- **Secret**: `keycloak-secrets` - Keycloak admin credentials
- **Secret**: `jwt-secrets` - JWT signing secrets

### Updating Configuration

To update configuration:

```bash
# Edit the ConfigMap
kubectl edit configmap sprint-config -n sprint-management

# Restart pods to pick up changes
kubectl rollout restart deployment/sprint-service -n sprint-management
```

## Monitoring and Observability

### Health Checks

Check service health:

```bash
# Sprint Service
kubectl exec -it deployment/sprint-service -n sprint-management -- curl http://localhost:8080/health/live
kubectl exec -it deployment/sprint-service -n sprint-management -- curl http://localhost:8080/health/ready

# Auth Service
kubectl exec -it deployment/auth-service -n sprint-management -- curl http://localhost:8080/health/live
kubectl exec -it deployment/auth-service -n sprint-management -- curl http://localhost:8080/health/ready
```

### Prometheus Metrics

Services expose Prometheus metrics at `/metrics`:

```bash
# Port-forward to access metrics
kubectl port-forward deployment/sprint-service 8080:8080 -n sprint-management

# Access metrics
curl http://localhost:8080/metrics
```

### Logs

View logs:

```bash
# Sprint Service logs
kubectl logs -f deployment/sprint-service -n sprint-management

# Auth Service logs
kubectl logs -f deployment/auth-service -n sprint-management

# PostgreSQL logs
kubectl logs -f deployment/postgres -n sprint-management

# NATS logs
kubectl logs -f deployment/nats -n sprint-management
```

### Pod Status

Check pod status:

```bash
# All pods in namespace
kubectl get pods -n sprint-management

# Detailed pod information
kubectl describe pod <pod-name> -n sprint-management

# Pod events
kubectl get events -n sprint-management --sort-by='.lastTimestamp'
```

## Scaling

### Manual Scaling

```bash
# Scale Sprint Service
kubectl scale deployment/sprint-service --replicas=5 -n sprint-management

# Scale Auth Service
kubectl scale deployment/auth-service --replicas=3 -n sprint-management
```

### Auto-Scaling

HorizontalPodAutoscaler is configured for both services:

- **Sprint Service**: 3-20 replicas based on 70% CPU, 80% memory
- **Auth Service**: 2-10 replicas based on 70% CPU, 80% memory

View HPA status:

```bash
kubectl get hpa -n sprint-management
kubectl describe hpa sprint-service-hpa -n sprint-management
```

## Database Management

### Migrations

Run database migrations:

```bash
# Port-forward to PostgreSQL
kubectl port-forward service/postgres-service 5432:5432 -n sprint-management

# Run migrations from local machine
cd go_sprint
make migrate-up

# Or run migrations from a job
kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: db-migrate
  namespace: sprint-management
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: ghcr.io/humoroushorse/go_sprint:latest
        command: ["/app/migrate", "up"]
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: database-secrets
              key: DATABASE_MASTER_URL
      restartPolicy: Never
  backoffLimit: 3
EOF
```

### Backups

Create database backup:

```bash
# Port-forward to PostgreSQL
kubectl port-forward service/postgres-service 5432:5432 -n sprint-management

# Create backup
pg_dump -h localhost -U postgres -d sprint_management > backup_$(date +%Y%m%d_%H%M%S).sql
```

For automated backups, consider using:
- [Velero](https://velero.io/) for cluster backups
- [Stash](https://stash.run/) for database backups
- CronJob with pg_dump

## Security

### Network Policies

Network policies are configured to restrict traffic:

- Sprint Service can access: PostgreSQL, NATS, Auth Service, Keycloak
- Auth Service can access: PostgreSQL, NATS, Keycloak
- PostgreSQL accepts connections only from services
- NATS accepts connections only from services

### Pod Security

All application pods run with:
- Non-root user (UID 1000)
- Read-only root filesystem
- Dropped capabilities
- No privilege escalation

### Secrets Management

**Production Recommendations**:

1. Use external secrets management (Vault, AWS Secrets Manager, etc.)
2. Enable encryption at rest for Kubernetes secrets
3. Rotate secrets regularly
4. Use RBAC to restrict secret access

## Troubleshooting

### Common Issues

#### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n sprint-management

# Check pod events
kubectl describe pod <pod-name> -n sprint-management

# Check logs
kubectl logs <pod-name> -n sprint-management
```

#### Database Connection Issues

```bash
# Test PostgreSQL connectivity
kubectl exec -it deployment/sprint-service -n sprint-management -- sh
# Inside pod:
nc -zv postgres-service 5432
```

#### NATS Connection Issues

```bash
# Check NATS status
kubectl exec -it deployment/nats -n sprint-management -- nats-server --signal status

# Test NATS connectivity
kubectl exec -it deployment/sprint-service -n sprint-management -- sh
# Inside pod:
nc -zv nats-service 4222
```

#### Service Not Accessible

```bash
# Check service endpoints
kubectl get endpoints -n sprint-management

# Check ingress
kubectl get ingress -n sprint-management
kubectl describe ingress sprint-management-ingress -n sprint-management

# Check ingress controller logs
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller
```

### Debug Mode

Enable debug logging:

```bash
kubectl set env deployment/sprint-service LOG_LEVEL=debug -n sprint-management
kubectl set env deployment/auth-service LOG_LEVEL=debug -n sprint-management
```

## Production Considerations

### High Availability

1. **Multiple Replicas**: Run at least 3 replicas of each service
2. **Pod Disruption Budgets**: Ensure minimum availability during updates
3. **Anti-Affinity**: Spread pods across nodes
4. **Database Replication**: Use PostgreSQL streaming replication
5. **NATS Clustering**: Deploy NATS in cluster mode

### Performance

1. **Resource Limits**: Tune based on actual usage
2. **Connection Pooling**: Configure database connection pools
3. **Caching**: Enable Redis for caching (optional)
4. **CDN**: Use CDN for static assets

### Monitoring

1. **Prometheus**: Collect metrics from all services
2. **Grafana**: Visualize metrics and create dashboards
3. **Alerting**: Configure alerts for critical issues
4. **Distributed Tracing**: Use Jaeger or Zipkin

### Backup and Recovery

1. **Database Backups**: Automated daily backups with retention
2. **Disaster Recovery**: Document and test recovery procedures
3. **Cluster Backups**: Use Velero for full cluster backups

## Cleanup

To remove all resources:

```bash
# Delete all resources in namespace
kubectl delete namespace sprint-management

# Or delete individual components
kubectl delete -f sprint-service-deployment.yaml
kubectl delete -f auth-service-deployment.yaml
kubectl delete -f keycloak-deployment.yaml
kubectl delete -f nats-deployment.yaml
kubectl delete -f postgres-deployment.yaml
kubectl delete -f ingress.yaml
kubectl delete -f network-policy.yaml
kubectl delete -f configmap.yaml
kubectl delete -f secrets.yaml
kubectl delete -f namespace.yaml
```

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [NATS on Kubernetes](https://docs.nats.io/running-a-nats-service/nats-kubernetes)
- [PostgreSQL on Kubernetes](https://www.postgresql.org/docs/current/high-availability.html)
- [Keycloak on Kubernetes](https://www.keycloak.org/server/containers)

## Support

For issues and questions:
- Check the main project README
- Review the troubleshooting section above
- Check application logs
- Review Kubernetes events
