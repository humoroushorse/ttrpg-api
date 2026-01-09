# Operational Runbook: Go Sprint Management Service

## Table of Contents

1. [Service Overview](#service-overview)
2. [Deployment Procedures](#deployment-procedures)
3. [Monitoring and Health Checks](#monitoring-and-health-checks)
4. [Common Operations](#common-operations)
5. [Incident Response](#incident-response)
6. [Maintenance Procedures](#maintenance-procedures)
7. [Scaling Guidelines](#scaling-guidelines)

## Service Overview

### Service Details

- **Service Name**: Sprint Management Service
- **Repository**: `go_sprint`
- **Language**: Go 1.21+
- **Port**: 8080 (HTTP), 8081 (Metrics)
- **Dependencies**: PostgreSQL, NATS, Auth Service (go_auth)

### Key Endpoints

- **Health Check (Liveness)**: `GET /health/live`
- **Health Check (Readiness)**: `GET /health/ready`
- **Metrics**: `GET /metrics`
- **API Base**: `/api/v1`
- **WebSocket**: `/ws`

### Service Dependencies

1. **PostgreSQL** (Required)
   - Master: Write operations
   - Replica: Read operations
   - Port: 5432

2. **NATS** (Required)
   - Message broker for inter-service communication
   - Port: 4222 (client), 8222 (monitoring)

3. **Auth Service** (Required)
   - JWT validation and user management
   - Port: 8081

4. **Keycloak** (Required via Auth Service)
   - Identity and access management
   - Port: 8080

## Deployment Procedures

### Pre-Deployment Checklist

- [ ] Database migrations tested in staging
- [ ] Configuration files reviewed and updated
- [ ] Backup created before deployment
- [ ] Rollback plan documented
- [ ] Monitoring alerts configured
- [ ] Team notified of deployment window

### Standard Deployment (Kubernetes)

```bash
# 1. Verify current deployment status
kubectl get pods -n sprint-management
kubectl get deployments -n sprint-management

# 2. Apply configuration changes
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secrets.yaml

# 3. Run database migrations
kubectl exec -it deployment/sprint-management -n sprint-management -- \
  ./main migrate up

# 4. Deploy new version
kubectl set image deployment/sprint-management \
  sprint-management=sprint-management:v1.2.3 \
  -n sprint-management

# 5. Monitor rollout
kubectl rollout status deployment/sprint-management -n sprint-management

# 6. Verify health
kubectl exec -it deployment/sprint-management -n sprint-management -- \
  curl http://localhost:8080/health/ready
```

### Rollback Procedure

```bash
# 1. Check rollout history
kubectl rollout history deployment/sprint-management -n sprint-management

# 2. Rollback to previous version
kubectl rollout undo deployment/sprint-management -n sprint-management

# 3. Rollback to specific revision
kubectl rollout undo deployment/sprint-management \
  --to-revision=2 \
  -n sprint-management

# 4. Verify rollback
kubectl rollout status deployment/sprint-management -n sprint-management

# 5. Run database migration rollback if needed
kubectl exec -it deployment/sprint-management -n sprint-management -- \
  ./main migrate down 1
```

### Docker Deployment

```bash
# 1. Pull latest image
docker pull sprint-management:latest

# 2. Stop current container
docker stop sprint-management

# 3. Backup database
./scripts/backup.sh

# 4. Run migrations
docker run --rm \
  --network sprint-network \
  -e DATABASE_MASTER_URL="$DATABASE_MASTER_URL" \
  sprint-management:latest \
  ./main migrate up

# 5. Start new container
docker run -d \
  --name sprint-management \
  --network sprint-network \
  -p 8080:8080 \
  -e DATABASE_MASTER_URL="$DATABASE_MASTER_URL" \
  -e DATABASE_REPLICA_URL="$DATABASE_REPLICA_URL" \
  -e NATS_URL="$NATS_URL" \
  sprint-management:latest

# 6. Verify health
curl http://localhost:8080/health/ready
```

## Monitoring and Health Checks

### Health Check Endpoints

#### Liveness Probe

```bash
# Check if service is alive
curl http://localhost:8080/health/live

# Expected response: 200 OK
# {"status": "ok", "timestamp": "2024-01-15T10:30:00Z"}
```

**Purpose**: Indicates if the service process is running
**Action on Failure**: Restart the container/pod

#### Readiness Probe

```bash
# Check if service is ready to handle requests
curl http://localhost:8080/health/ready

# Expected response: 200 OK
# {
#   "status": "ready",
#   "checks": {
#     "database": "ok",
#     "nats": "ok"
#   },
#   "timestamp": "2024-01-15T10:30:00Z"
# }
```

**Purpose**: Indicates if service can handle traffic
**Action on Failure**: Remove from load balancer, investigate dependencies

### Key Metrics to Monitor

#### Application Metrics

```bash
# View all metrics
curl http://localhost:8081/metrics

# Key metrics to monitor:
# - http_requests_total
# - http_request_duration_seconds
# - database_queries_total
# - database_query_duration_seconds
# - nats_messages_published_total
# - nats_messages_received_total
# - work_items_created_total
# - sprint_velocity_points
# - active_sprints_count
```

#### Prometheus Queries

```promql
# Request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Database connection pool usage
database_connections_in_use / database_connections_max

# Active WebSocket connections
websocket_connections_active
```

### Log Monitoring

#### Log Locations

- **Kubernetes**: `kubectl logs -f deployment/sprint-management -n sprint-management`
- **Docker**: `docker logs -f sprint-management`
- **Local**: stdout (structured JSON in production)

#### Important Log Patterns

```bash
# Error logs
kubectl logs deployment/sprint-management -n sprint-management | grep '"level":"ERROR"'

# Slow queries (>1s)
kubectl logs deployment/sprint-management -n sprint-management | \
  grep '"duration"' | awk '$NF > 1000'

# Authentication failures
kubectl logs deployment/sprint-management -n sprint-management | \
  grep '"msg":"authentication failed"'

# Database connection errors
kubectl logs deployment/sprint-management -n sprint-management | \
  grep '"error":"database"'
```

## Common Operations

### Database Operations

#### Run Migrations

```bash
# Up migrations
./main migrate up

# Down migrations (rollback one)
./main migrate down 1

# Check migration status
./main migrate version

# Force migration version (use with caution)
./main migrate force 5
```

#### Create Backup

```bash
# Quick database backup
make backup

# Comprehensive system backup
make backup-system

# Backup with custom path
BACKUP_DIR=/mnt/backups ./scripts/backup.sh
```

#### Restore from Backup

```bash
# List available backups
make backup-list

# Restore specific backup
make restore BACKUP_FILE=./backups/sprint_management_20240115_120000.sql

# Verify restoration
make verify-system
```

### NATS Operations

#### Check NATS Connection

```bash
# View NATS monitoring endpoint
curl http://nats-server:8222/varz

# Check connections
curl http://nats-server:8222/connz

# Check subscriptions
curl http://nats-server:8222/subsz
```

#### Publish Test Message

```bash
# Using nats CLI
nats pub sprint.test-trace-id.workitem.create.request '{"title":"Test"}'

# Subscribe to all sprint messages
nats sub "sprint.>"
```

### Cache Operations

#### Clear Cache

```bash
# Clear all caches (if Redis is used)
redis-cli FLUSHALL

# Clear specific cache keys
redis-cli DEL "sprint:*"
```

### User Management

#### Sync Users from Keycloak

```bash
# Trigger user sync via NATS
nats pub auth.sync-trace-id.user.sync.request '{}'

# Or via API
curl -X POST http://auth-service:8080/api/v1/users/sync \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## Incident Response

### High Error Rate

**Symptoms**: 5xx errors increasing, error rate > 5%

**Investigation Steps**:

1. Check service health:
   ```bash
   curl http://localhost:8080/health/ready
   ```

2. Check recent logs for errors:
   ```bash
   kubectl logs --tail=100 deployment/sprint-management -n sprint-management | \
     grep '"level":"ERROR"'
   ```

3. Check database connectivity:
   ```bash
   kubectl exec -it deployment/sprint-management -n sprint-management -- \
     psql $DATABASE_MASTER_URL -c "SELECT 1"
   ```

4. Check NATS connectivity:
   ```bash
   curl http://nats-server:8222/connz | jq '.connections[] | select(.name | contains("sprint"))'
   ```

**Resolution**:

- If database issue: Check connection pool, restart database if needed
- If NATS issue: Restart NATS or check network connectivity
- If application issue: Check recent deployments, consider rollback

### Slow Response Times

**Symptoms**: P95 latency > 500ms, P99 latency > 1s

**Investigation Steps**:

1. Check slow queries:
   ```bash
   kubectl logs deployment/sprint-management -n sprint-management | \
     grep '"duration"' | sort -k8 -n | tail -20
   ```

2. Check database performance:
   ```sql
   -- Connect to database
   SELECT query, mean_exec_time, calls
   FROM pg_stat_statements
   ORDER BY mean_exec_time DESC
   LIMIT 10;
   ```

3. Check connection pool:
   ```bash
   curl http://localhost:8081/metrics | grep database_connections
   ```

**Resolution**:

- Add database indexes for slow queries
- Increase connection pool size if exhausted
- Scale horizontally if CPU/memory constrained
- Enable query caching for read-heavy endpoints

### Database Connection Exhaustion

**Symptoms**: "too many connections" errors, connection pool exhausted

**Investigation Steps**:

1. Check current connections:
   ```sql
   SELECT count(*) FROM pg_stat_activity;
   SELECT max_connections FROM pg_settings WHERE name='max_connections';
   ```

2. Check connection pool metrics:
   ```bash
   curl http://localhost:8081/metrics | grep database_connections
   ```

3. Identify long-running queries:
   ```sql
   SELECT pid, now() - query_start as duration, query
   FROM pg_stat_activity
   WHERE state = 'active'
   ORDER BY duration DESC;
   ```

**Resolution**:

- Increase `max_connections` in PostgreSQL
- Increase connection pool size in application config
- Kill long-running queries if necessary:
  ```sql
  SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE pid = <pid>;
  ```
- Scale application horizontally to distribute load

### NATS Connection Issues

**Symptoms**: Messages not being delivered, NATS connection errors

**Investigation Steps**:

1. Check NATS server status:
   ```bash
   curl http://nats-server:8222/varz
   ```

2. Check service NATS connection:
   ```bash
   kubectl logs deployment/sprint-management -n sprint-management | \
     grep '"nats"'
   ```

3. Test NATS connectivity:
   ```bash
   nats pub test.subject "test message"
   nats sub test.subject
   ```

**Resolution**:

- Restart NATS server if unhealthy
- Check network connectivity between services
- Verify NATS URL configuration
- Check NATS authentication credentials

### Memory Leak

**Symptoms**: Memory usage continuously increasing, OOM kills

**Investigation Steps**:

1. Check memory usage:
   ```bash
   kubectl top pods -n sprint-management
   ```

2. Get heap profile:
   ```bash
   curl http://localhost:8081/debug/pprof/heap > heap.prof
   go tool pprof heap.prof
   ```

3. Check goroutine count:
   ```bash
   curl http://localhost:8081/debug/pprof/goroutine?debug=1
   ```

**Resolution**:

- Restart service to free memory (short-term)
- Analyze heap profile to identify leak source
- Check for unclosed database connections
- Check for goroutine leaks (WebSocket connections, etc.)
- Deploy fix and monitor memory usage

## Maintenance Procedures

### Routine Maintenance

#### Daily Tasks

- [ ] Review error logs for anomalies
- [ ] Check backup completion status
- [ ] Monitor key metrics (error rate, latency, throughput)
- [ ] Verify all health checks passing

#### Weekly Tasks

- [ ] Review and clean up old backups
- [ ] Analyze slow query logs
- [ ] Review security alerts
- [ ] Check disk space usage
- [ ] Update dependencies if needed

#### Monthly Tasks

- [ ] Review and optimize database indexes
- [ ] Analyze performance trends
- [ ] Review and update documentation
- [ ] Conduct disaster recovery drill
- [ ] Review and update monitoring alerts

### Database Maintenance

#### Vacuum and Analyze

```sql
-- Vacuum all tables
VACUUM ANALYZE;

-- Vacuum specific table
VACUUM ANALYZE sprint_management.work_items;

-- Check table bloat
SELECT schemaname, tablename, 
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'sprint_management'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

#### Reindex

```sql
-- Reindex all tables in schema
REINDEX SCHEMA sprint_management;

-- Reindex specific table
REINDEX TABLE sprint_management.work_items;
```

#### Clean Up Soft Deleted Items

```bash
# Permanently delete items older than retention period
curl -X POST http://localhost:8080/api/v1/admin/cleanup-deleted \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"retention_days": 30}'
```

### Log Rotation

```bash
# If using file-based logging
# Configure logrotate
cat > /etc/logrotate.d/sprint-management << EOF
/var/log/sprint-management/*.log {
    daily
    rotate 30
    compress
    delaycompress
    notifempty
    create 0640 sprint-management sprint-management
    sharedscripts
    postrotate
        systemctl reload sprint-management
    endscript
}
EOF
```

## Scaling Guidelines

### Horizontal Scaling

#### When to Scale

- CPU usage consistently > 70%
- Memory usage consistently > 80%
- Request queue depth increasing
- P95 latency > 500ms

#### Kubernetes Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: sprint-management-hpa
  namespace: sprint-management
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: sprint-management
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

#### Manual Scaling

```bash
# Scale to 5 replicas
kubectl scale deployment/sprint-management --replicas=5 -n sprint-management

# Verify scaling
kubectl get pods -n sprint-management
```

### Vertical Scaling

#### Increase Resource Limits

```yaml
# Update deployment
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "2000m"
```

```bash
# Apply changes
kubectl apply -f k8s/sprint-service-deployment.yaml
```

### Database Scaling

#### Read Replica Setup

1. Configure PostgreSQL replication
2. Update application configuration:
   ```yaml
   database:
     master_url: postgres://master:5432/sprint_management
     replica_url: postgres://replica:5432/sprint_management
   ```
3. Restart application to use replica for reads

#### Connection Pool Tuning

```yaml
database:
  max_open_conns: 50      # Increase for high load
  max_idle_conns: 10      # Keep some connections ready
  conn_max_lifetime: 5m   # Recycle connections
```

## Emergency Contacts

### On-Call Rotation

- **Primary**: [On-call engineer]
- **Secondary**: [Backup engineer]
- **Manager**: [Engineering manager]

### Escalation Path

1. On-call engineer (immediate)
2. Team lead (15 minutes)
3. Engineering manager (30 minutes)
4. Director of Engineering (1 hour)

### Communication Channels

- **Slack**: #sprint-management-alerts
- **PagerDuty**: sprint-management-service
- **Email**: sprint-management-team@company.com

## Additional Resources

- [API Documentation](./api-usage-example.md)
- [Database Setup](./database-setup.md)
- [Backup and Restore](./backup-restore.md)
- [Disaster Recovery](./disaster-recovery.md)
- [Configuration Guide](./configuration-and-logging.md)
- [Kubernetes Deployment](./kubernetes-deployment.md)
- [Docker Deployment](./docker-deployment.md)
