# NATS Deployment Configuration

This directory contains deployment configurations for NATS, the message broker shared by the ttrpg-api services (go_sprint, and go_dnd's tabletop).

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

## Subjects

Every subject is listed in [`../../docs/nats-subjects.md`](../../docs/nats-subjects.md), and the naming
rules are in `.kiro/steering/nats-messaging-patterns.md`: `service.entity.{id}.action`, with the trace id
in the message, not the subject.

An earlier version of this README put the trace id in the subject (`sprint.{trace_id}.workitem.create.request`).
That makes a new subject for every request, so nothing can subscribe to "this board's events" without a
wildcard over every trace. Don't copy it.

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

One NATS for every service, container `ttrpg-nats`, in the `ttrpg` compose project with postgres and
keycloak (`nats.compose.yml`). Its settings are all in `nats.conf`: nats-server takes only a few of them
as command-line flags (`--max_payload`, for one, isn't), and an unknown flag makes it exit at once.

```bash
# from any service that includes ../makefiles/nats.mk
make nats-up     # starts it and waits for /healthz
make nats-down   # stops it; postgres and keycloak keep running
make nats-logs
make nats-clean  # deletes JetStream's store (~/projects/ttrpg/volume-mounts/nats)
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

`nats.compose.yml` runs the server with `nats.conf`. Check a config change with
`docker run --rm -v "$PWD/nats.conf:/etc/nats/nats.conf:ro" nats:2.15.3-alpine -c /etc/nats/nats.conf -t`.

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
docker ps --filter name=ttrpg-nats

# Check NATS logs
docker logs ttrpg-nats

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
