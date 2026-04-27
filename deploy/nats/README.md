# NATS Deployment Configuration

This directory contains deployment configurations for NATS, the message broker used for inter-service communication in the sprint management system.

## Overview

NATS is deployed as shared infrastructure, similar to PostgreSQL and Keycloak. All microservices use the same NATS instance for messaging.

## Architecture

```
┌─────────────────┐
│   NATS Cluster  │
│   Port: 4222    │
│   Monitoring:   │
│   Port: 8222    │
└─────────────────┘
         │
         ├──────────► Auth Service
         │            (auth.{trace_id}.*.*)
         │
         └──────────► Sprint Service
                      (sprint.{trace_id}.*.*.*)
```

## Subject Pattern Standards

All services follow a standardized subject pattern for consistency and traceability:

### Pattern Format

```
{service}.{trace_id}.{resource}.{action}.{status}
```

### Components

- **service**: Service name (e.g., `auth`, `sprint`)
- **trace_id**: UUID for request tracing and correlation
- **resource**: Resource being operated on (e.g., `workitem`, `sprint`, `user`)
- **action**: Operation being performed (e.g., `create`, `update`, `delete`, `validate`)
- **status**: Message type (e.g., `request`, `response`, `error`)

### Examples

#### Auth Service Subjects

```
auth.{trace_id}.validate.request
auth.{trace_id}.validate.response
auth.{trace_id}.user.sync.request
auth.{trace_id}.user.sync.response
```

#### Sprint Service Subjects

```
sprint.{trace_id}.workitem.create.request
sprint.{trace_id}.workitem.create.response
sprint.{trace_id}.sprint.close.request
sprint.{trace_id}.sprint.close.response
sprint.{trace_id}.notification.workitem.created
```

### Wildcard Subscriptions

For tracing and monitoring, use wildcard patterns:

```
# All messages for a specific trace
sprint.{trace_id}.*.*.*
auth.{trace_id}.*.*

# All messages for a service
sprint.>
auth.>

# All notifications
*.*.notification.*
```

## Message Headers

All NATS messages include standard headers:

```go
headers := nats.Header{
    "Trace-ID":       traceID,
    "Correlation-ID": correlationID,
    "Timestamp":      time.Now().UTC().Format(time.RFC3339),
    "Service":        "sprint",
    "Version":        "v1",
}
```

## Deployment Options

### Local Development (Docker Compose)

```bash
# Start NATS
make nats-up

# Stop NATS
make nats-down

# View logs
make nats-logs
```

### Kubernetes

```bash
# Deploy NATS
kubectl apply -f deploy/nats/k8s/

# Check status
kubectl get pods -l app=nats

# View logs
kubectl logs -l app=nats -f
```

## Configuration

### Docker Compose

See `docker-compose.yml` for local development configuration.

### Kubernetes

See `k8s/` directory for production deployment manifests.

## Monitoring

NATS exposes monitoring endpoints:

- **HTTP Monitoring**: http://localhost:8222
- **Metrics**: http://localhost:8222/varz
- **Connections**: http://localhost:8222/connz
- **Routes**: http://localhost:8222/routez
- **Subscriptions**: http://localhost:8222/subsz

## Service-Specific Documentation

Each service documents its specific NATS subjects:

- **Auth Service**: [go_auth/docs/nats.md](../../go_auth/docs/nats.md)
- **Sprint Service**: [go_sprint/docs/nats.md](../../go_sprint/docs/nats.md)

## Best Practices

### Subject Design

1. **Use hierarchical subjects**: Organize subjects in a logical hierarchy
2. **Include trace IDs**: Always include trace ID for request correlation
3. **Be specific**: Use descriptive resource and action names
4. **Avoid deep nesting**: Keep subject depth reasonable (4-5 levels max)

### Message Design

1. **Include metadata**: Always include trace ID, timestamp, and correlation ID
2. **Use JSON**: Serialize messages as JSON for interoperability
3. **Version messages**: Include version information for backward compatibility
4. **Keep messages small**: Avoid large payloads; use references instead

### Error Handling

1. **Use error subjects**: Publish errors to `.error` subjects
2. **Include error details**: Provide actionable error information
3. **Log errors**: Always log errors with trace correlation
4. **Implement retries**: Use exponential backoff for transient failures

### Performance

1. **Use request-reply**: For synchronous operations, use NATS request-reply
2. **Batch when possible**: Group related operations
3. **Monitor queue depth**: Watch for message buildup
4. **Set timeouts**: Always set reasonable timeouts for requests

## Security

### Authentication

NATS supports multiple authentication methods:

- **Token-based**: Simple token authentication
- **Username/Password**: Basic authentication
- **TLS**: Certificate-based authentication
- **JWT**: JSON Web Token authentication

For production, use JWT or TLS authentication.

### Authorization

Configure subject-level permissions:

```
# Example authorization config
authorization {
  users = [
    {
      user: "sprint-service"
      password: "$2a$11$..."
      permissions: {
        publish: ["sprint.>"]
        subscribe: ["sprint.>", "auth.*.validate.response"]
      }
    }
  ]
}
```

## Troubleshooting

### Connection Issues

```bash
# Check NATS is running
docker ps | grep nats

# Check NATS logs
docker logs nats

# Test connection
nats-cli server check
```

### Message Delivery Issues

```bash
# Check subscriptions
curl http://localhost:8222/subsz

# Check connections
curl http://localhost:8222/connz

# Monitor messages
nats-cli sub "sprint.>"
```

### Performance Issues

```bash
# Check server stats
curl http://localhost:8222/varz

# Monitor slow consumers
curl http://localhost:8222/connz?subs=1

# Check message rates
curl http://localhost:8222/varz | jq '.in_msgs, .out_msgs'
```

## References

- [NATS Documentation](https://docs.nats.io/)
- [NATS Subject Design](https://docs.nats.io/nats-concepts/subjects)
- [NATS Request-Reply](https://docs.nats.io/nats-concepts/reqreply)
- [NATS Monitoring](https://docs.nats.io/running-a-nats-service/nats_admin/monitoring)
