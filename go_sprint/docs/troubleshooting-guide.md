# Troubleshooting Guide: Go Sprint Management Service

## Table of Contents

1. [Quick Diagnostics](#quick-diagnostics)
2. [Common Issues](#common-issues)
3. [Database Issues](#database-issues)
4. [NATS Issues](#nats-issues)
5. [Authentication Issues](#authentication-issues)
6. [Performance Issues](#performance-issues)
7. [WebSocket Issues](#websocket-issues)
8. [Deployment Issues](#deployment-issues)
9. [Data Integrity Issues](#data-integrity-issues)
10. [Debugging Tools](#debugging-tools)

## Quick Diagnostics

### Health Check Script

```bash
#!/bin/bash
# quick-health-check.sh

echo "=== Sprint Management Service Health Check ==="
echo

# Check service health
echo "1. Service Health:"
curl -s http://localhost:8080/health/ready | jq .
echo

# Check database connectivity
echo "2. Database Connectivity:"
psql $DATABASE_MASTER_URL -c "SELECT 1" > /dev/null 2>&1 && echo "✓ Master DB: OK" || echo "✗ Master DB: FAILED"
psql $DATABASE_REPLICA_URL -c "SELECT 1" > /dev/null 2>&1 && echo "✓ Replica DB: OK" || echo "✗ Replica DB: FAILED"
echo

# Check NATS connectivity
echo "3. NATS Connectivity:"
curl -s http://nats-server:8222/varz > /dev/null 2>&1 && echo "✓ NATS: OK" || echo "✗ NATS: FAILED"
echo

# Check Auth Service
echo "4. Auth Service:"
curl -s http://auth-service:8080/health/ready > /dev/null 2>&1 && echo "✓ Auth Service: OK" || echo "✗ Auth Service: FAILED"
echo

# Check recent errors
echo "5. Recent Errors (last 10):"
kubectl logs --tail=100 deployment/sprint-management -n sprint-management 2>/dev/null | \
  grep '"level":"ERROR"' | tail -10 || echo "No recent errors"
echo

# Check metrics
echo "6. Key Metrics:"
curl -s http://localhost:8081/metrics | grep -E "(http_requests_total|database_connections|active_sprints)" | head -5
echo

echo "=== Health Check Complete ==="
```

### Service Status Check

```bash
# Kubernetes
kubectl get pods -n sprint-management
kubectl describe pod <pod-name> -n sprint-management
kubectl logs <pod-name> -n sprint-management --tail=50

# Docker
docker ps | grep sprint-management
docker logs sprint-management --tail=50
docker inspect sprint-management

# Local
ps aux | grep sprint-management
netstat -tulpn | grep 8080
```

## Common Issues

### Issue: Service Won't Start

**Symptoms**:
- Container/pod crashes immediately
- "CrashLoopBackOff" in Kubernetes
- Exit code 1 or 2

**Diagnosis**:

```bash
# Check logs
kubectl logs <pod-name> -n sprint-management

# Check configuration
kubectl get configmap sprint-management-config -n sprint-management -o yaml
kubectl get secret sprint-management-secrets -n sprint-management -o yaml

# Check environment variables
kubectl exec -it <pod-name> -n sprint-management -- env | grep -E "(DATABASE|NATS|AUTH)"
```

**Common Causes**:

1. **Missing Environment Variables**
   ```bash
   # Verify required variables are set
   echo $DATABASE_MASTER_URL
   echo $DATABASE_REPLICA_URL
   echo $NATS_URL
   echo $AUTH_SERVICE_URL
   ```

2. **Invalid Configuration**
   ```bash
   # Validate config file
   cat config.yaml
   # Check for syntax errors, missing required fields
   ```

3. **Database Connection Failure**
   ```bash
   # Test database connection
   psql $DATABASE_MASTER_URL -c "SELECT 1"
   ```

**Resolution**:

```bash
# Fix missing environment variables
kubectl edit deployment sprint-management -n sprint-management

# Update configuration
kubectl apply -f k8s/configmap.yaml

# Restart service
kubectl rollout restart deployment/sprint-management -n sprint-management
```

### Issue: 500 Internal Server Error

**Symptoms**:
- API returns 500 status code
- Error logs show panic or unhandled errors

**Diagnosis**:

```bash
# Check recent error logs
kubectl logs deployment/sprint-management -n sprint-management | \
  grep '"level":"ERROR"' | tail -20

# Check for panics
kubectl logs deployment/sprint-management -n sprint-management | \
  grep -i "panic"

# Check trace ID for specific request
kubectl logs deployment/sprint-management -n sprint-management | \
  grep "trace_id\":\"<trace-id>\""
```

**Common Causes**:

1. **Database Query Error**
   - Invalid SQL syntax
   - Missing table or column
   - Constraint violation

2. **Nil Pointer Dereference**
   - Missing null checks
   - Uninitialized variables

3. **External Service Failure**
   - Auth service down
   - NATS connection lost

**Resolution**:

```bash
# Check database schema
psql $DATABASE_MASTER_URL -c "\dt sprint_management.*"

# Verify migrations are up to date
kubectl exec -it deployment/sprint-management -n sprint-management -- \
  ./main migrate version

# Check external services
curl http://auth-service:8080/health/ready
curl http://nats-server:8222/varz
```

### Issue: 401 Unauthorized

**Symptoms**:
- API returns 401 status code
- "Invalid token" or "Token expired" errors

**Diagnosis**:

```bash
# Check JWT token
echo $JWT_TOKEN | cut -d'.' -f2 | base64 -d | jq .

# Check token expiration
echo $JWT_TOKEN | cut -d'.' -f2 | base64 -d | jq '.exp' | \
  xargs -I {} date -d @{}

# Check auth service logs
kubectl logs deployment/auth-service -n sprint-management | \
  grep "validate"
```

**Common Causes**:

1. **Expired Token**
   - Token TTL exceeded
   - Clock skew between services

2. **Invalid Token**
   - Malformed JWT
   - Wrong signing key

3. **Auth Service Down**
   - Cannot validate tokens
   - NATS communication failure

**Resolution**:

```bash
# Get new token from Keycloak
curl -X POST http://keycloak:8080/realms/sprint-management/protocol/openid-connect/token \
  -d "client_id=sprint-client" \
  -d "client_secret=$CLIENT_SECRET" \
  -d "grant_type=client_credentials"

# Verify auth service is running
kubectl get pods -n sprint-management | grep auth-service

# Check NATS connectivity between services
nats sub "auth.>"
```

### Issue: 404 Not Found

**Symptoms**:
- API returns 404 for valid endpoints
- "Route not found" errors

**Diagnosis**:

```bash
# Check API routes
curl http://localhost:8080/api/v1/workitems
curl http://localhost:8080/api/v1/sprints

# Check OpenAPI spec
cat api/openapi/combined.yaml | grep -A 5 "paths:"

# Check handler registration
kubectl logs deployment/sprint-management -n sprint-management | \
  grep "Registered route"
```

**Common Causes**:

1. **Incorrect URL Path**
   - Missing `/api/v1` prefix
   - Typo in endpoint path

2. **Handler Not Registered**
   - Code generation issue
   - Missing route registration

3. **Load Balancer Misconfiguration**
   - Incorrect path routing
   - Missing ingress rules

**Resolution**:

```bash
# Verify API documentation
cat docs/api-usage-example.md

# Regenerate API code
make generate-api

# Check ingress configuration
kubectl get ingress -n sprint-management
kubectl describe ingress sprint-management -n sprint-management
```

## Database Issues

### Issue: Connection Pool Exhausted

**Symptoms**:
- "too many connections" errors
- Slow response times
- Timeouts on database operations

**Diagnosis**:

```sql
-- Check current connections
SELECT count(*) as current_connections,
       (SELECT setting::int FROM pg_settings WHERE name='max_connections') as max_connections
FROM pg_stat_activity;

-- Check connections by state
SELECT state, count(*) 
FROM pg_stat_activity 
GROUP BY state;

-- Check long-running queries
SELECT pid, now() - query_start as duration, state, query
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY duration DESC
LIMIT 10;
```

**Resolution**:

```bash
# Increase connection pool size
kubectl edit configmap sprint-management-config -n sprint-management
# Update: database.max_open_conns: 50

# Increase PostgreSQL max_connections
# Edit postgresql.conf: max_connections = 200

# Kill long-running queries
psql $DATABASE_MASTER_URL -c "SELECT pg_terminate_backend(<pid>);"

# Restart service to reset connections
kubectl rollout restart deployment/sprint-management -n sprint-management
```

### Issue: Slow Queries

**Symptoms**:
- High database query duration in logs
- Slow API response times
- High database CPU usage

**Diagnosis**:

```sql
-- Enable pg_stat_statements if not already enabled
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Find slow queries
SELECT query, 
       mean_exec_time, 
       calls, 
       total_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Check missing indexes
SELECT schemaname, tablename, attname, n_distinct, correlation
FROM pg_stats
WHERE schemaname = 'sprint_management'
  AND n_distinct > 100
  AND correlation < 0.1;

-- Check table sizes
SELECT schemaname, tablename,
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'sprint_management'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

**Resolution**:

```sql
-- Add missing indexes
CREATE INDEX CONCURRENTLY idx_work_items_assignee 
ON sprint_management.work_items(assignee_id) 
WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY idx_work_items_sprint 
ON sprint_management.work_items(sprint_id) 
WHERE deleted_at IS NULL;

-- Analyze tables
ANALYZE sprint_management.work_items;
ANALYZE sprint_management.sprints;

-- Vacuum if needed
VACUUM ANALYZE sprint_management.work_items;
```

### Issue: Migration Failures

**Symptoms**:
- Migration command fails
- Database schema out of sync
- "dirty database" errors

**Diagnosis**:

```bash
# Check migration status
./main migrate version

# Check migration files
ls -la migrations/

# Check schema_migrations table
psql $DATABASE_MASTER_URL -c "SELECT * FROM schema_migrations;"
```

**Resolution**:

```bash
# Force migration version (use with caution)
./main migrate force <version>

# Rollback and retry
./main migrate down 1
./main migrate up

# Manual fix if needed
psql $DATABASE_MASTER_URL -f migrations/<migration-file>.up.sql
```

## NATS Issues

### Issue: Messages Not Being Delivered

**Symptoms**:
- NATS publish succeeds but no subscribers receive messages
- "No responders" errors
- Timeout waiting for responses

**Diagnosis**:

```bash
# Check NATS server status
curl http://nats-server:8222/varz | jq .

# Check connections
curl http://nats-server:8222/connz | jq '.connections[] | select(.name | contains("sprint"))'

# Check subscriptions
curl http://nats-server:8222/subsz | jq .

# Test message flow
nats pub sprint.test-trace.workitem.create.request '{"test": true}'
nats sub "sprint.>"
```

**Common Causes**:

1. **Subject Pattern Mismatch**
   - Publisher and subscriber using different patterns
   - Typo in subject name

2. **No Active Subscribers**
   - Service not running
   - Subscription not registered

3. **NATS Server Issues**
   - Server overloaded
   - Network connectivity problems

**Resolution**:

```bash
# Verify subject patterns
cat docs/nats.md

# Check service subscriptions
kubectl logs deployment/sprint-management -n sprint-management | \
  grep "Subscribed to"

# Restart NATS server
kubectl rollout restart deployment/nats -n sprint-management

# Restart services
kubectl rollout restart deployment/sprint-management -n sprint-management
kubectl rollout restart deployment/auth-service -n sprint-management
```

### Issue: NATS Connection Drops

**Symptoms**:
- "Connection closed" errors
- Reconnection attempts in logs
- Intermittent message delivery

**Diagnosis**:

```bash
# Check NATS server logs
kubectl logs deployment/nats -n sprint-management

# Check connection errors
kubectl logs deployment/sprint-management -n sprint-management | \
  grep "nats.*error"

# Check network connectivity
kubectl exec -it deployment/sprint-management -n sprint-management -- \
  nc -zv nats-server 4222
```

**Resolution**:

```bash
# Increase NATS timeouts
kubectl edit configmap sprint-management-config -n sprint-management
# Update: nats.reconnect_wait: 5s
#         nats.max_reconnects: 20

# Check NATS resource limits
kubectl describe pod <nats-pod> -n sprint-management

# Scale NATS if needed
kubectl scale deployment/nats --replicas=3 -n sprint-management
```

## Authentication Issues

### Issue: JWT Validation Failures

**Symptoms**:
- "Invalid signature" errors
- "Token validation failed" in logs
- 401 responses for valid tokens

**Diagnosis**:

```bash
# Decode JWT token
echo $JWT_TOKEN | cut -d'.' -f2 | base64 -d | jq .

# Check token claims
echo $JWT_TOKEN | cut -d'.' -f2 | base64 -d | jq '{exp, iat, iss, sub}'

# Check auth service logs
kubectl logs deployment/auth-service -n sprint-management | \
  grep "validate"

# Test token validation
curl -X POST http://auth-service:8080/api/v1/auth/validate \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Common Causes**:

1. **Wrong Public Key**
   - Keycloak realm key changed
   - Key not synced to auth service

2. **Clock Skew**
   - Time difference between services
   - Token appears expired or not yet valid

3. **Token Tampering**
   - Modified token
   - Invalid signature

**Resolution**:

```bash
# Sync Keycloak public key
curl http://keycloak:8080/realms/sprint-management/protocol/openid-connect/certs

# Update auth service configuration
kubectl edit configmap auth-service-config -n sprint-management

# Sync system clocks
kubectl exec -it deployment/sprint-management -n sprint-management -- date
kubectl exec -it deployment/auth-service -n sprint-management -- date

# Restart auth service
kubectl rollout restart deployment/auth-service -n sprint-management
```

### Issue: User Not Found

**Symptoms**:
- "User not found" errors
- Valid JWT but user doesn't exist in database
- 404 on user-related operations

**Diagnosis**:

```sql
-- Check if user exists
SELECT * FROM auth.users WHERE keycloak_id = '<keycloak-id>';

-- Check user sync status
SELECT * FROM auth.users ORDER BY updated_at DESC LIMIT 10;
```

**Resolution**:

```bash
# Trigger user sync
curl -X POST http://auth-service:8080/api/v1/users/sync \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Manual user creation
psql $DATABASE_MASTER_URL << EOF
INSERT INTO auth.users (keycloak_id, username, email, first_name, last_name)
VALUES ('<keycloak-id>', '<username>', '<email>', '<first>', '<last>');
EOF
```

## Performance Issues

### Issue: High Memory Usage

**Symptoms**:
- Memory usage continuously increasing
- OOMKilled pods in Kubernetes
- Slow garbage collection

**Diagnosis**:

```bash
# Check memory usage
kubectl top pods -n sprint-management

# Get heap profile
kubectl port-forward deployment/sprint-management 6060:6060 -n sprint-management
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Check goroutine count
curl http://localhost:6060/debug/pprof/goroutine?debug=1

# Check for memory leaks
curl http://localhost:6060/debug/pprof/allocs > allocs.prof
go tool pprof allocs.prof
```

**Common Causes**:

1. **Goroutine Leak**
   - Unclosed WebSocket connections
   - Blocked goroutines

2. **Memory Leak**
   - Unclosed database connections
   - Growing caches without eviction

3. **Large Response Payloads**
   - Fetching too much data
   - No pagination

**Resolution**:

```bash
# Increase memory limits
kubectl edit deployment sprint-management -n sprint-management
# Update: resources.limits.memory: 2Gi

# Enable GC tuning
kubectl edit deployment sprint-management -n sprint-management
# Add env: GOGC=50

# Restart service
kubectl rollout restart deployment/sprint-management -n sprint-management

# Fix code issues and redeploy
```

### Issue: High CPU Usage

**Symptoms**:
- CPU usage consistently > 80%
- Slow request processing
- CPU throttling in Kubernetes

**Diagnosis**:

```bash
# Check CPU usage
kubectl top pods -n sprint-management

# Get CPU profile
kubectl port-forward deployment/sprint-management 6060:6060 -n sprint-management
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Check for hot paths
go tool pprof -http=:8080 cpu.prof
```

**Common Causes**:

1. **Inefficient Algorithms**
   - N+1 queries
   - Unnecessary loops

2. **JSON Marshaling**
   - Large payloads
   - Frequent serialization

3. **High Request Volume**
   - Need horizontal scaling

**Resolution**:

```bash
# Increase CPU limits
kubectl edit deployment sprint-management -n sprint-management
# Update: resources.limits.cpu: 2000m

# Scale horizontally
kubectl scale deployment/sprint-management --replicas=5 -n sprint-management

# Optimize code and redeploy
```

## WebSocket Issues

### Issue: WebSocket Connections Dropping

**Symptoms**:
- Clients frequently disconnecting
- "Connection closed" errors
- No real-time updates

**Diagnosis**:

```bash
# Check active WebSocket connections
curl http://localhost:8081/metrics | grep websocket_connections

# Check WebSocket logs
kubectl logs deployment/sprint-management -n sprint-management | \
  grep "websocket"

# Test WebSocket connection
wscat -c ws://localhost:8080/ws?token=$JWT_TOKEN
```

**Common Causes**:

1. **Load Balancer Timeout**
   - Idle timeout too short
   - No keep-alive

2. **Authentication Expiry**
   - JWT token expired
   - No token refresh

3. **Network Issues**
   - Firewall blocking WebSocket
   - Proxy not supporting WebSocket

**Resolution**:

```bash
# Increase load balancer timeout
kubectl edit ingress sprint-management -n sprint-management
# Add annotation: nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"

# Enable WebSocket keep-alive
kubectl edit configmap sprint-management-config -n sprint-management
# Update: websocket.ping_interval: 30s

# Configure ingress for WebSocket
kubectl apply -f - << EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: sprint-management
  annotations:
    nginx.ingress.kubernetes.io/websocket-services: sprint-management
spec:
  rules:
  - host: sprint.example.com
    http:
      paths:
      - path: /ws
        pathType: Prefix
        backend:
          service:
            name: sprint-management
            port:
              number: 8080
EOF
```

### Issue: Messages Not Broadcasting

**Symptoms**:
- WebSocket connected but no messages received
- Updates not appearing in real-time
- Broadcast failures in logs

**Diagnosis**:

```bash
# Check WebSocket hub status
kubectl logs deployment/sprint-management -n sprint-management | \
  grep "hub"

# Check NATS integration
kubectl logs deployment/sprint-management -n sprint-management | \
  grep "notification"

# Test message broadcasting
curl -X POST http://localhost:8080/api/v1/workitems \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Test","type":"story"}'
```

**Resolution**:

```bash
# Restart WebSocket hub
kubectl rollout restart deployment/sprint-management -n sprint-management

# Check room subscriptions
# Ensure clients are subscribed to correct rooms

# Verify NATS notification publishing
nats sub "sprint.*.notification.*"
```

## Deployment Issues

### Issue: Rolling Update Fails

**Symptoms**:
- Deployment stuck in progress
- New pods not becoming ready
- Old pods not terminating

**Diagnosis**:

```bash
# Check rollout status
kubectl rollout status deployment/sprint-management -n sprint-management

# Check pod status
kubectl get pods -n sprint-management

# Check pod events
kubectl describe pod <pod-name> -n sprint-management

# Check readiness probe
kubectl logs <pod-name> -n sprint-management
curl http://<pod-ip>:8080/health/ready
```

**Resolution**:

```bash
# Pause rollout
kubectl rollout pause deployment/sprint-management -n sprint-management

# Fix issues (update config, fix health checks, etc.)

# Resume rollout
kubectl rollout resume deployment/sprint-management -n sprint-management

# Or rollback
kubectl rollout undo deployment/sprint-management -n sprint-management
```

### Issue: Image Pull Errors

**Symptoms**:
- "ImagePullBackOff" or "ErrImagePull"
- Pods not starting
- Image not found errors

**Diagnosis**:

```bash
# Check pod events
kubectl describe pod <pod-name> -n sprint-management | grep -A 10 Events

# Check image name
kubectl get deployment sprint-management -n sprint-management -o yaml | grep image:

# Test image pull
docker pull <image-name>
```

**Resolution**:

```bash
# Fix image name
kubectl set image deployment/sprint-management \
  sprint-management=correct-registry/sprint-management:v1.2.3 \
  -n sprint-management

# Add image pull secret if needed
kubectl create secret docker-registry regcred \
  --docker-server=<registry> \
  --docker-username=<username> \
  --docker-password=<password> \
  -n sprint-management

kubectl patch deployment sprint-management \
  -p '{"spec":{"template":{"spec":{"imagePullSecrets":[{"name":"regcred"}]}}}}' \
  -n sprint-management
```

## Data Integrity Issues

### Issue: Orphaned Records

**Symptoms**:
- Foreign key constraint violations
- References to non-existent records
- Data inconsistencies

**Diagnosis**:

```sql
-- Find orphaned work items (invalid sprint_id)
SELECT wi.id, wi.title, wi.sprint_id
FROM sprint_management.work_items wi
LEFT JOIN sprint_management.sprints s ON wi.sprint_id = s.id
WHERE wi.sprint_id IS NOT NULL AND s.id IS NULL;

-- Find orphaned dependencies
SELECT wd.id, wd.source_id, wd.target_id
FROM sprint_management.work_item_dependencies wd
LEFT JOIN sprint_management.work_items wi1 ON wd.source_id = wi1.id
LEFT JOIN sprint_management.work_items wi2 ON wd.target_id = wi2.id
WHERE wi1.id IS NULL OR wi2.id IS NULL;

-- Find orphaned comments
SELECT c.id, c.work_item_id
FROM sprint_management.comments c
LEFT JOIN sprint_management.work_items wi ON c.work_item_id = wi.id
WHERE wi.id IS NULL;
```

**Resolution**:

```sql
-- Clean up orphaned work items
UPDATE sprint_management.work_items
SET sprint_id = NULL
WHERE sprint_id NOT IN (SELECT id FROM sprint_management.sprints);

-- Delete orphaned dependencies
DELETE FROM sprint_management.work_item_dependencies
WHERE source_id NOT IN (SELECT id FROM sprint_management.work_items)
   OR target_id NOT IN (SELECT id FROM sprint_management.work_items);

-- Delete orphaned comments
DELETE FROM sprint_management.comments
WHERE work_item_id NOT IN (SELECT id FROM sprint_management.work_items);
```

### Issue: Soft Delete Inconsistencies

**Symptoms**:
- Deleted items appearing in queries
- Unable to restore soft-deleted items
- Retention period not enforced

**Diagnosis**:

```sql
-- Check soft-deleted items
SELECT id, title, deleted_at, deleted_by
FROM sprint_management.work_items
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC;

-- Check items past retention period
SELECT id, title, deleted_at,
       NOW() - deleted_at as age
FROM sprint_management.work_items
WHERE deleted_at IS NOT NULL
  AND deleted_at < NOW() - INTERVAL '30 days';
```

**Resolution**:

```bash
# Run cleanup job
curl -X POST http://localhost:8080/api/v1/admin/cleanup-deleted \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"retention_days": 30}'

# Or manual cleanup
psql $DATABASE_MASTER_URL << EOF
DELETE FROM sprint_management.work_items
WHERE deleted_at IS NOT NULL
  AND deleted_at < NOW() - INTERVAL '30 days';
EOF
```

## Debugging Tools

### Enable Debug Logging

```bash
# Kubernetes
kubectl set env deployment/sprint-management LOG_LEVEL=debug -n sprint-management

# Docker
docker run -e LOG_LEVEL=debug sprint-management:latest

# Local
export LOG_LEVEL=debug
./main
```

### pprof Profiling

```bash
# Start pprof server (already enabled in service)
kubectl port-forward deployment/sprint-management 6060:6060 -n sprint-management

# CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Heap profile
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Goroutine profile
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof

# Interactive web UI
go tool pprof -http=:8080 cpu.prof
```

### Distributed Tracing

```bash
# Follow trace across services
kubectl logs deployment/sprint-management -n sprint-management | \
  grep "trace_id\":\"<trace-id>\""

kubectl logs deployment/auth-service -n sprint-management | \
  grep "trace_id\":\"<trace-id>\""

# NATS message tracing
nats sub "sprint.<trace-id>.>"
nats sub "auth.<trace-id>.>"
```

### Database Query Analysis

```sql
-- Enable query logging
ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET log_duration = on;
SELECT pg_reload_conf();

-- View slow queries
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 20;

-- Explain query plan
EXPLAIN ANALYZE
SELECT * FROM sprint_management.work_items
WHERE assignee_id = '<user-id>'
  AND deleted_at IS NULL;
```

### Network Debugging

```bash
# Test connectivity
kubectl exec -it deployment/sprint-management -n sprint-management -- \
  nc -zv postgres-master 5432

kubectl exec -it deployment/sprint-management -n sprint-management -- \
  nc -zv nats-server 4222

# DNS resolution
kubectl exec -it deployment/sprint-management -n sprint-management -- \
  nslookup postgres-master

# Packet capture
kubectl exec -it deployment/sprint-management -n sprint-management -- \
  tcpdump -i any -w /tmp/capture.pcap port 5432
```

## Getting Help

### Log Collection

```bash
# Collect all relevant logs
./scripts/collect-logs.sh > debug-logs.txt

# Or manually
kubectl logs deployment/sprint-management -n sprint-management --tail=1000 > sprint-service.log
kubectl logs deployment/auth-service -n sprint-management --tail=1000 > auth-service.log
kubectl logs deployment/nats -n sprint-management --tail=1000 > nats.log
kubectl describe pod <pod-name> -n sprint-management > pod-description.txt
```

### Support Ticket Information

When creating a support ticket, include:

1. **Service version**: `kubectl get deployment sprint-management -n sprint-management -o yaml | grep image:`
2. **Error logs**: Last 100 lines with ERROR level
3. **Trace ID**: For specific failed requests
4. **Health check output**: `/health/ready` response
5. **Metrics snapshot**: Key metrics from `/metrics`
6. **Recent changes**: Deployments, config changes, migrations
7. **Environment**: Kubernetes version, cluster size, resource limits

### Additional Resources

- [Operational Runbook](./operational-runbook.md)
- [API Documentation](./api-usage-example.md)
- [Configuration Guide](./configuration-and-logging.md)
- [Backup and Restore](./backup-restore.md)
- [Disaster Recovery](./disaster-recovery.md)
