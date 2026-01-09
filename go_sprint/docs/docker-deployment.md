# Docker Deployment Guide

This guide covers deploying the Sprint Management System using Docker and Docker Compose.

## Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- 4GB RAM minimum
- 10GB disk space

## Quick Start (Development)

1. **Clone the repositories**:
```bash
git clone https://github.com/humoroushorse/go_auth
git clone https://github.com/humoroushorse/go_sprint
```

2. **Start all services**:
```bash
cd go_sprint
docker-compose up -d
```

3. **Check service health**:
```bash
docker-compose ps
```

4. **View logs**:
```bash
docker-compose logs -f sprint-service
```

## Production Deployment

### 1. Environment Configuration

Copy the example environment file and configure it:

```bash
cp .env.example .env
```

Edit `.env` with production values:
- Set strong passwords for `POSTGRES_PASSWORD` and `KEYCLOAK_ADMIN_PASSWORD`
- Configure `KEYCLOAK_HOSTNAME` with your domain
- Set `CORS_ALLOWED_ORIGINS` to your frontend URL
- Configure `DOCKER_REGISTRY` if using a private registry

### 2. Build Images

Build production images:

```bash
# Build auth service
cd ../go_auth
docker build -t ghcr.io/humoroushorse/go_auth:v1.0.0 .

# Build sprint service
cd ../go_sprint
docker build -t ghcr.io/humoroushorse/go_sprint:v1.0.0 .
```

### 3. Push to Registry

```bash
docker push ghcr.io/humoroushorse/go_auth:v1.0.0
docker push ghcr.io/humoroushorse/go_sprint:v1.0.0
```

### 4. Deploy with Production Configuration

```bash
docker-compose -f docker-compose.prod.yml up -d
```

## Service Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Docker Network                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────┐    ┌──────────────┐                 │
│  │  PostgreSQL  │◄───┤   Keycloak   │                 │
│  │   :5432      │    │    :8080     │                 │
│  └──────────────┘    └──────────────┘                 │
│         ▲                    ▲                         │
│         │                    │                         │
│  ┌──────┴──────┐    ┌───────┴──────┐                 │
│  │ Auth Service│◄───┤ Sprint Service│                 │
│  │   :8081     │    │     :8082     │                 │
│  └─────────────┘    └───────────────┘                 │
│         ▲                    ▲                         │
│         │                    │                         │
│         └────────┬───────────┘                         │
│                  │                                     │
│         ┌────────▼────────┐                           │
│         │      NATS       │                           │
│         │     :4222       │                           │
│         └─────────────────┘                           │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Service Endpoints

| Service | Port | Endpoint | Description |
|---------|------|----------|-------------|
| Sprint Service | 8082 | http://localhost:8082 | Main API |
| Auth Service | 8081 | http://localhost:8081 | Authentication API |
| Keycloak | 8080 | http://localhost:8080 | Identity Provider |
| PostgreSQL | 5432 | localhost:5432 | Database |
| NATS | 4222 | nats://localhost:4222 | Message Broker |
| NATS Monitoring | 8222 | http://localhost:8222 | NATS Dashboard |

## Health Checks

All services include health check endpoints:

```bash
# Sprint Service
curl http://localhost:8082/health/live
curl http://localhost:8082/health/ready

# Auth Service
curl http://localhost:8081/health/live
curl http://localhost:8081/health/ready

# Keycloak
curl http://localhost:8080/health/ready

# NATS
curl http://localhost:8222/healthz
```

## Database Initialization

The database is automatically initialized with:
- `sprint_management` schema
- `auth` schema
- Required tables and indexes
- Initial migrations

To manually run migrations:

```bash
docker-compose exec sprint-service /app/sprint-server migrate up
```

## Monitoring and Logs

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f sprint-service

# Last 100 lines
docker-compose logs --tail=100 sprint-service
```

### Metrics

Prometheus metrics are available at:
- Sprint Service: http://localhost:8082/metrics
- Auth Service: http://localhost:8081/metrics

### NATS Monitoring

Access NATS monitoring dashboard:
- URL: http://localhost:8222
- View connections, subscriptions, and message stats

## Backup and Restore

### Database Backup

```bash
# Create backup
docker-compose exec postgres pg_dump \
  -U postgres \
  -d sprint_management \
  --schema=sprint_management \
  --schema=auth \
  > backup_$(date +%Y%m%d_%H%M%S).sql

# Or use the backup script
docker-compose exec sprint-service /app/scripts/backup.sh
```

### Restore Database

```bash
# Restore from backup
docker-compose exec -T postgres psql \
  -U postgres \
  -d sprint_management \
  < backup_20240115_120000.sql
```

## Troubleshooting

### Service Won't Start

1. Check logs:
```bash
docker-compose logs sprint-service
```

2. Verify dependencies are healthy:
```bash
docker-compose ps
```

3. Check resource usage:
```bash
docker stats
```

### Database Connection Issues

1. Verify PostgreSQL is running:
```bash
docker-compose exec postgres pg_isready -U postgres
```

2. Check connection string in environment variables:
```bash
docker-compose exec sprint-service env | grep DATABASE
```

### NATS Connection Issues

1. Check NATS is running:
```bash
curl http://localhost:8222/healthz
```

2. View NATS connections:
```bash
curl http://localhost:8222/connz
```

### Keycloak Issues

1. Check Keycloak logs:
```bash
docker-compose logs keycloak
```

2. Verify realm configuration:
- Access admin console: http://localhost:8080
- Login with admin credentials
- Check realm: sprint-management

## Scaling

### Horizontal Scaling

Scale sprint service replicas:

```bash
docker-compose up -d --scale sprint-service=3
```

Note: Requires load balancer configuration (see Kubernetes deployment for production scaling).

### Resource Limits

Resource limits are configured in `docker-compose.prod.yml`:

```yaml
deploy:
  resources:
    limits:
      cpus: '2'
      memory: 1G
    reservations:
      cpus: '1'
      memory: 512M
```

## Security Best Practices

1. **Use secrets management**: Don't commit `.env` files
2. **Enable TLS**: Use reverse proxy (nginx/traefik) for HTTPS
3. **Network isolation**: Use Docker networks to isolate services
4. **Non-root users**: All services run as non-root (UID 1000)
5. **Regular updates**: Keep base images updated
6. **Scan images**: Use `docker scan` to check for vulnerabilities

## Cleanup

### Stop Services

```bash
docker-compose down
```

### Remove Volumes (WARNING: Deletes all data)

```bash
docker-compose down -v
```

### Remove Images

```bash
docker rmi ghcr.io/humoroushorse/go_sprint:latest
docker rmi ghcr.io/humoroushorse/go_auth:latest
```

## Next Steps

- [Kubernetes Deployment](./kubernetes-deployment.md)
- [Configuration Guide](./configuration-and-logging.md)
- [API Documentation](./api-usage-example.md)
- [Monitoring Setup](../pkg/metrics/README.md)
