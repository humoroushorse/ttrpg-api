# Disaster Recovery Procedures

## Overview

This document outlines the disaster recovery procedures for the Sprint Management System. It covers backup strategies, restoration procedures, and testing protocols to ensure business continuity.

## Backup Strategy

### Automated Backups

The system supports automated backups through the backup scheduler:

```go
// Configure automated backups
config := backup.BackupConfig{
    Schedule:      "0 2 * * *",  // Daily at 2 AM
    RetentionDays: 30,            // Keep 30 days
    BackupPath:    "/backups",
    Host:          "localhost",
    Port:          "5432",
    Username:      "postgres",
    Database:      "sprint_management",
    IncludeSchema: true,
    IncludeData:   true,
}

scheduler := backup.NewBackupScheduler(config, logger)
scheduler.Start(ctx, 24*time.Hour)
```

### Manual Backups

#### Database Only

```bash
# Full database backup
./scripts/backup.sh

# Schema only
BACKUP_TYPE=schema-only ./scripts/backup.sh

# Data only
BACKUP_TYPE=data-only ./scripts/backup.sh

# With custom retention
RETENTION_DAYS=60 ./scripts/backup.sh
```

#### Comprehensive System Backup

```bash
# Backup everything (database, config, docs, migrations, API specs)
./scripts/backup-system.sh

# Custom backup location
BACKUP_ROOT=/mnt/backups ./scripts/backup-system.sh
```

### Backup Components

A comprehensive backup includes:

1. **Database**
   - `sprint_management` schema
   - `auth` schema
   - All tables, indexes, and constraints

2. **Configuration Files**
   - `docker-compose.yml`
   - `Makefile`
   - `sqlc.yaml`
   - `go.mod` and `go.sum`
   - `Dockerfile`

3. **Database Migrations**
   - All migration files (up and down)
   - Migration history

4. **OpenAPI Specifications**
   - All API definition files
   - Generated code configuration

5. **Documentation**
   - Technical documentation
   - API documentation
   - Architecture diagrams

6. **NATS Configuration**
   - Subject patterns
   - Message schemas
   - Deployment configuration

## Restoration Procedures

### Pre-Restoration Checklist

Before restoring from backup:

1. ✅ Verify backup integrity (checksum validation)
2. ✅ Ensure sufficient disk space
3. ✅ Stop all running services
4. ✅ Create a pre-restore backup of current state
5. ✅ Notify all users of maintenance window
6. ✅ Document the reason for restoration

### Database Restoration

#### Quick Restore

```bash
# Restore from backup file
./scripts/restore.sh ./backups/sprint_management_20240115_120000.sql
```

#### Manual Restore

```bash
# 1. Verify backup integrity
sha256sum -c sprint_management_20240115_120000.sql.sha256

# 2. Create pre-restore backup
pg_dump -h localhost -U postgres -d sprint_management \
  --schema=sprint_management --schema=auth \
  -f pre_restore_backup.sql

# 3. Restore database
psql -h localhost -U postgres -d sprint_management \
  -f sprint_management_20240115_120000.sql

# 4. Verify restoration
psql -h localhost -U postgres -d sprint_management \
  -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema IN ('sprint_management', 'auth');"
```

### Full System Restoration

```bash
# 1. Extract backup archive
tar -xzf system_backup_20240115_120000.tar.gz

# 2. Review manifest
cat system_backup_20240115_120000/MANIFEST.txt

# 3. Restore database
psql -h localhost -U postgres -d sprint_management \
  -f system_backup_20240115_120000/database/sprint_management_20240115_120000.sql

# 4. Restore configuration files
cp system_backup_20240115_120000/config/* ./

# 5. Restore migrations (if needed)
cp -r system_backup_20240115_120000/migrations/* ./migrations/

# 6. Restore API specifications
cp -r system_backup_20240115_120000/api/* ./api/

# 7. Restore documentation
cp -r system_backup_20240115_120000/docs/* ./docs/

# 8. Verify NATS configuration
diff system_backup_20240115_120000/nats/nats.md docs/nats.md

# 9. Run migrations if needed
make migrate-up

# 10. Restart services
docker-compose up -d
```

## Disaster Scenarios

### Scenario 1: Database Corruption

**Symptoms:**
- Database connection errors
- Data inconsistencies
- Failed queries

**Recovery Steps:**

1. Stop all services accessing the database
2. Identify the last known good backup
3. Verify backup integrity
4. Restore database from backup
5. Run database integrity checks
6. Restart services
7. Verify system functionality

**Commands:**

```bash
# Stop services
docker-compose down

# Restore database
./scripts/restore.sh ./backups/sprint_management_YYYYMMDD_HHMMSS.sql

# Verify integrity
psql -h localhost -U postgres -d sprint_management -c "
  SELECT schemaname, tablename, n_live_tup 
  FROM pg_stat_user_tables 
  WHERE schemaname IN ('sprint_management', 'auth');"

# Restart services
docker-compose up -d

# Verify functionality
curl http://localhost:8080/health/ready
```

### Scenario 2: Complete System Failure

**Symptoms:**
- All services down
- Infrastructure failure
- Data center outage

**Recovery Steps:**

1. Provision new infrastructure
2. Install dependencies (PostgreSQL, NATS, Docker)
3. Restore from comprehensive backup
4. Verify all components
5. Update DNS/load balancer
6. Monitor system health

**Commands:**

```bash
# 1. Install dependencies
# (OS-specific commands)

# 2. Extract backup
tar -xzf system_backup_YYYYMMDD_HHMMSS.tar.gz
cd system_backup_YYYYMMDD_HHMMSS

# 3. Restore database
createdb -h localhost -U postgres sprint_management
psql -h localhost -U postgres -d sprint_management \
  -f database/sprint_management_YYYYMMDD_HHMMSS.sql

# 4. Restore configuration
cp config/* /path/to/project/

# 5. Deploy services
cd /path/to/project
docker-compose up -d

# 6. Verify
./scripts/verify-system.sh
```

### Scenario 3: Accidental Data Deletion

**Symptoms:**
- Missing work items, sprints, or comments
- User reports of lost data

**Recovery Steps:**

1. Identify the time of deletion
2. Find the most recent backup before deletion
3. Extract specific data from backup
4. Restore only the affected data
5. Verify data integrity

**Commands:**

```bash
# Extract specific table data
pg_restore -h localhost -U postgres -d sprint_management \
  --table=sprint_management.work_items \
  sprint_management_YYYYMMDD_HHMMSS.sql

# Or use selective restore
psql -h localhost -U postgres -d sprint_management << EOF
BEGIN;
-- Restore specific records
INSERT INTO sprint_management.work_items 
SELECT * FROM backup_work_items WHERE id IN (...);
COMMIT;
EOF
```

### Scenario 4: Configuration Loss

**Symptoms:**
- Services fail to start
- Missing environment variables
- Incorrect settings

**Recovery Steps:**

1. Extract configuration from backup
2. Review and update environment-specific settings
3. Restore configuration files
4. Restart services

**Commands:**

```bash
# Extract configuration
tar -xzf system_backup_YYYYMMDD_HHMMSS.tar.gz \
  system_backup_YYYYMMDD_HHMMSS/config

# Restore configuration
cp system_backup_YYYYMMDD_HHMMSS/config/* ./

# Update environment-specific settings
vi docker-compose.yml
vi .env

# Restart
docker-compose up -d
```

## Testing Procedures

### Regular Backup Testing

Perform backup restoration tests monthly:

```bash
#!/bin/bash
# Monthly backup test script

# 1. Create test environment
docker-compose -f docker-compose.test.yml up -d

# 2. Restore latest backup
LATEST_BACKUP=$(ls -t backups/sprint_management_*.sql | head -1)
./scripts/restore.sh "$LATEST_BACKUP"

# 3. Run verification queries
psql -h localhost -U postgres -d sprint_management_test << EOF
-- Verify table counts
SELECT 'work_items' as table_name, COUNT(*) as count FROM sprint_management.work_items
UNION ALL
SELECT 'sprints', COUNT(*) FROM sprint_management.sprints
UNION ALL
SELECT 'comments', COUNT(*) FROM sprint_management.comments;

-- Verify data integrity
SELECT COUNT(*) as orphaned_work_items 
FROM sprint_management.work_items 
WHERE parent_id IS NOT NULL 
  AND parent_id NOT IN (SELECT id FROM sprint_management.work_items);
EOF

# 4. Cleanup test environment
docker-compose -f docker-compose.test.yml down -v
```

### Disaster Recovery Drill

Perform full disaster recovery drills quarterly:

1. **Week 1: Planning**
   - Schedule drill date
   - Notify team
   - Prepare test environment

2. **Week 2: Execution**
   - Simulate disaster scenario
   - Execute recovery procedures
   - Document time to recovery

3. **Week 3: Review**
   - Analyze results
   - Identify improvements
   - Update procedures

4. **Week 4: Implementation**
   - Apply improvements
   - Update documentation
   - Train team

## Recovery Time Objectives (RTO)

| Scenario | Target RTO | Maximum RTO |
|----------|-----------|-------------|
| Database corruption | 1 hour | 4 hours |
| Single service failure | 15 minutes | 1 hour |
| Complete system failure | 4 hours | 24 hours |
| Data deletion | 2 hours | 8 hours |
| Configuration loss | 30 minutes | 2 hours |

## Recovery Point Objectives (RPO)

| Data Type | Target RPO | Maximum RPO |
|-----------|-----------|-------------|
| Database | 1 hour | 24 hours |
| Configuration | 1 day | 7 days |
| Documentation | 1 week | 1 month |

## Backup Retention Policy

| Backup Type | Retention Period | Storage Location |
|-------------|-----------------|------------------|
| Daily backups | 30 days | Local disk |
| Weekly backups | 90 days | Network storage |
| Monthly backups | 1 year | Cloud storage |
| Yearly backups | 7 years | Archive storage |

## Monitoring and Alerts

### Backup Monitoring

Monitor backup health:

```bash
# Check last backup age
LAST_BACKUP=$(ls -t backups/sprint_management_*.sql | head -1)
BACKUP_AGE=$(($(date +%s) - $(stat -c %Y "$LAST_BACKUP")))
if [ $BACKUP_AGE -gt 86400 ]; then
  echo "WARNING: Last backup is older than 24 hours"
fi

# Check backup size
BACKUP_SIZE=$(stat -c %s "$LAST_BACKUP")
if [ $BACKUP_SIZE -lt 1000000 ]; then
  echo "WARNING: Backup size is suspiciously small"
fi
```

### Alert Configuration

Configure alerts for:

- ❌ Backup failure
- ❌ Backup age > 24 hours
- ❌ Backup size anomaly
- ❌ Backup validation failure
- ❌ Insufficient disk space

## Contact Information

### Emergency Contacts

| Role | Name | Contact |
|------|------|---------|
| Database Administrator | [Name] | [Phone/Email] |
| System Administrator | [Name] | [Phone/Email] |
| DevOps Lead | [Name] | [Phone/Email] |
| On-Call Engineer | [Rotation] | [Phone/Email] |

### Escalation Path

1. **Level 1**: On-call engineer (0-30 minutes)
2. **Level 2**: System administrator (30-60 minutes)
3. **Level 3**: Database administrator (1-2 hours)
4. **Level 4**: Engineering manager (2+ hours)

## Appendix

### Useful Commands

```bash
# List all backups
ls -lh backups/sprint_management_*.sql

# Check backup integrity
sha256sum -c backups/sprint_management_*.sql.sha256

# Estimate restore time
wc -l backups/sprint_management_YYYYMMDD_HHMMSS.sql

# Monitor restore progress
tail -f /var/log/postgresql/postgresql.log

# Verify database after restore
psql -h localhost -U postgres -d sprint_management \
  -c "\dt sprint_management.*" \
  -c "\dt auth.*"
```

### Troubleshooting

**Problem**: Restore fails with "relation already exists"

**Solution**: Drop existing tables or use `--clean` flag

```bash
psql -h localhost -U postgres -d sprint_management \
  -c "DROP SCHEMA sprint_management CASCADE; DROP SCHEMA auth CASCADE;"
```

**Problem**: Backup file is corrupted

**Solution**: Use previous backup or restore from archive

```bash
# Try previous backup
PREV_BACKUP=$(ls -t backups/sprint_management_*.sql | sed -n 2p)
./scripts/restore.sh "$PREV_BACKUP"
```

**Problem**: Insufficient disk space

**Solution**: Clean up old backups or expand storage

```bash
# Remove backups older than 60 days
find backups/ -name "sprint_management_*.sql" -mtime +60 -delete
```

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2024-01-15 | System | Initial version |

## Review Schedule

This document should be reviewed and updated:
- After each disaster recovery drill
- When procedures change
- Quarterly at minimum
- After any actual disaster recovery event
