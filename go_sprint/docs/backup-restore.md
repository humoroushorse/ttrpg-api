# Backup and Restore Guide

## Overview

The Sprint Management System includes comprehensive backup and restore capabilities to ensure data protection and business continuity. This guide covers all backup strategies, restoration procedures, and best practices.

## Quick Start

### Create a Backup

```bash
# Database only
make backup

# Complete system backup (database + config + docs)
make backup-system

# List available backups
make backup-list
```

### Restore from Backup

```bash
# Restore database
make restore BACKUP_FILE=./backups/sprint_management_20240115_120000.sql

# Verify system after restore
make verify-system
```

## Backup Types

### 1. Database Backup

Backs up only the PostgreSQL database (both `sprint_management` and `auth` schemas).

**Usage:**

```bash
# Full backup (schema + data)
./scripts/backup.sh

# Schema only
BACKUP_TYPE=schema-only ./scripts/backup.sh

# Data only
BACKUP_TYPE=data-only ./scripts/backup.sh

# Custom retention
RETENTION_DAYS=60 ./scripts/backup.sh
```

**Output:**
- `backups/sprint_management_YYYYMMDD_HHMMSS.sql`
- `backups/sprint_management_YYYYMMDD_HHMMSS.sql.sha256` (checksum)

### 2. Comprehensive System Backup

Backs up everything needed for complete system recovery:
- Database (both schemas)
- Configuration files
- Database migrations
- OpenAPI specifications
- Documentation
- NATS configuration

**Usage:**

```bash
./scripts/backup-system.sh

# Custom backup location
BACKUP_ROOT=/mnt/backups ./scripts/backup-system.sh
```

**Output:**
- `backups/system_backup_YYYYMMDD_HHMMSS/` (directory)
- `backups/system_backup_YYYYMMDD_HHMMSS.tar.gz` (compressed archive)
- `backups/system_backup_YYYYMMDD_HHMMSS.tar.gz.sha256` (checksum)

**Directory Structure:**

```
system_backup_YYYYMMDD_HHMMSS/
├── database/
│   └── sprint_management_YYYYMMDD_HHMMSS.sql
├── config/
│   ├── docker-compose.yml
│   ├── Makefile
│   ├── sqlc.yaml
│   ├── go.mod
│   └── Dockerfile
├── migrations/
│   └── *.sql
├── api/
│   └── openapi/*.yaml
├── docs/
│   └── *.md
├── nats/
│   ├── nats.md
│   ├── subjects.txt
│   └── deployment configs
├── MANIFEST.txt
└── README.md
```

### 3. Automated Backups

Configure automated backups using the backup scheduler:

```go
import "github.com/humoroushorse/go_sprint/pkg/backup"

config := backup.BackupConfig{
    Schedule:      "0 2 * * *",  // Daily at 2 AM
    RetentionDays: 30,            // Keep 30 days
    BackupPath:    "/backups",
    Host:          "localhost",
    Port:          "5432",
    Username:      "postgres",
    Password:      os.Getenv("DB_PASSWORD"),
    Database:      "sprint_management",
    IncludeSchema: true,
    IncludeData:   true,
}

scheduler := backup.NewBackupScheduler(config, logger)
scheduler.Start(ctx, 24*time.Hour)
defer scheduler.Stop()
```

## Restoration Procedures

### Database Restoration

#### Quick Restore

```bash
./scripts/restore.sh ./backups/sprint_management_20240115_120000.sql
```

The script will:
1. Verify backup integrity (checksum)
2. Create a pre-restore backup
3. Prompt for confirmation
4. Restore the database
5. Verify the restoration

#### Manual Restore

```bash
# 1. Verify checksum
sha256sum -c sprint_management_20240115_120000.sql.sha256

# 2. Create pre-restore backup
pg_dump -h localhost -U postgres -d sprint_management \
  --schema=sprint_management --schema=auth \
  -f pre_restore_backup.sql

# 3. Restore
psql -h localhost -U postgres -d sprint_management \
  -f sprint_management_20240115_120000.sql

# 4. Verify
psql -h localhost -U postgres -d sprint_management \
  -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema IN ('sprint_management', 'auth');"
```

### Full System Restoration

```bash
# 1. Extract backup
tar -xzf system_backup_20240115_120000.tar.gz
cd system_backup_20240115_120000

# 2. Review manifest
cat MANIFEST.txt

# 3. Restore database
psql -h localhost -U postgres -d sprint_management \
  -f database/sprint_management_20240115_120000.sql

# 4. Restore configuration
cp config/* ../

# 5. Restore other components
cp -r migrations/* ../migrations/
cp -r api/* ../api/
cp -r docs/* ../docs/

# 6. Verify
cd ..
./scripts/verify-system.sh
```

## Backup Validation

### Automatic Validation

The backup scheduler automatically validates backups:
- File exists
- File is not empty
- File is readable
- File contains valid SQL content

### Manual Validation

```bash
# Verify checksum
sha256sum -c sprint_management_20240115_120000.sql.sha256

# Check file size
ls -lh sprint_management_20240115_120000.sql

# Verify SQL content
head -n 20 sprint_management_20240115_120000.sql

# Test restore to temporary database
createdb -h localhost -U postgres test_restore
psql -h localhost -U postgres -d test_restore \
  -f sprint_management_20240115_120000.sql
dropdb -h localhost -U postgres test_restore
```

## System Verification

After any restoration, verify system integrity:

```bash
./scripts/verify-system.sh
```

The verification script checks:
- ✅ Database connectivity
- ✅ Required schemas exist
- ✅ Core tables present
- ✅ No orphaned records
- ✅ Database indexes
- ✅ Configuration files
- ✅ Migration files
- ✅ API specifications
- ✅ Running services
- ✅ API health endpoints
- ✅ NATS configuration
- ✅ Documentation
- ✅ Data integrity
- ✅ Timestamp consistency
- ✅ Sprint date validity

## Backup Retention

### Default Retention Policy

| Backup Type | Retention Period |
|-------------|-----------------|
| Daily backups | 30 days |
| Weekly backups | 90 days |
| Monthly backups | 1 year |

### Custom Retention

```bash
# Set custom retention when creating backup
RETENTION_DAYS=60 ./scripts/backup.sh

# Configure in scheduler
config := backup.BackupConfig{
    RetentionDays: 60,  // Keep 60 days
    // ...
}
```

### Manual Cleanup

```bash
# Remove backups older than 30 days
find backups/ -name "sprint_management_*.sql" -mtime +30 -delete
find backups/ -name "*.sha256" -mtime +30 -delete

# Remove specific backup
rm backups/sprint_management_20240101_120000.sql*
```

## Environment Variables

### Backup Scripts

```bash
# Database connection
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=sprint_management
export DB_USER=postgres
export PGPASSWORD=your_password

# Backup configuration
export BACKUP_DIR=./backups
export BACKUP_ROOT=./backups
export BACKUP_TYPE=full  # full, schema-only, data-only
export RETENTION_DAYS=30
```

### Restore Scripts

```bash
# Database connection
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=sprint_management
export DB_USER=postgres
export PGPASSWORD=your_password
```

## Best Practices

### 1. Regular Backups

- Schedule daily automated backups
- Perform weekly comprehensive system backups
- Test restoration procedures monthly

### 2. Backup Storage

- Store backups on separate physical storage
- Use network storage or cloud storage for off-site backups
- Encrypt backups containing sensitive data
- Maintain multiple backup copies (3-2-1 rule)

### 3. Backup Testing

```bash
# Monthly backup test
#!/bin/bash
LATEST_BACKUP=$(ls -t backups/sprint_management_*.sql | head -1)

# Create test database
createdb -h localhost -U postgres test_restore

# Restore to test database
psql -h localhost -U postgres -d test_restore -f "$LATEST_BACKUP"

# Verify
psql -h localhost -U postgres -d test_restore -c "
  SELECT schemaname, tablename, n_live_tup 
  FROM pg_stat_user_tables 
  WHERE schemaname IN ('sprint_management', 'auth');"

# Cleanup
dropdb -h localhost -U postgres test_restore
```

### 4. Security

- Restrict backup file permissions: `chmod 600 backups/*.sql`
- Use encrypted connections for remote backups
- Store database passwords securely (environment variables, secrets manager)
- Audit backup access logs

### 5. Documentation

- Document backup procedures
- Maintain restoration runbooks
- Record backup schedules
- Track backup locations

## Troubleshooting

### Problem: Backup fails with "pg_dump: command not found"

**Solution:** Install PostgreSQL client tools

```bash
# macOS
brew install postgresql

# Ubuntu/Debian
sudo apt-get install postgresql-client

# RHEL/CentOS
sudo yum install postgresql
```

### Problem: Restore fails with "relation already exists"

**Solution:** Drop existing schemas or use clean restore

```bash
# Drop schemas
psql -h localhost -U postgres -d sprint_management -c "
  DROP SCHEMA IF EXISTS sprint_management CASCADE;
  DROP SCHEMA IF EXISTS auth CASCADE;"

# Then restore
./scripts/restore.sh backup_file.sql
```

### Problem: Backup file is corrupted

**Solution:** Verify checksum and use previous backup

```bash
# Check integrity
sha256sum -c sprint_management_20240115_120000.sql.sha256

# If corrupted, use previous backup
PREV_BACKUP=$(ls -t backups/sprint_management_*.sql | sed -n 2p)
./scripts/restore.sh "$PREV_BACKUP"
```

### Problem: Insufficient disk space

**Solution:** Clean up old backups or expand storage

```bash
# Check disk space
df -h

# Remove old backups
find backups/ -name "sprint_management_*.sql" -mtime +60 -delete

# Or move to archive storage
mv backups/sprint_management_2023*.sql /archive/
```

### Problem: Backup takes too long

**Solution:** Use parallel backup or optimize

```bash
# Use parallel backup (PostgreSQL 9.3+)
pg_dump -h localhost -U postgres -d sprint_management \
  --schema=sprint_management --schema=auth \
  --format=directory --jobs=4 \
  --file=backup_dir

# Or backup specific tables
pg_dump -h localhost -U postgres -d sprint_management \
  --schema=sprint_management \
  --table=work_items --table=sprints \
  -f partial_backup.sql
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Backup Database

on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM
  workflow_dispatch:

jobs:
  backup:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Install PostgreSQL client
        run: sudo apt-get install -y postgresql-client
      
      - name: Create backup
        env:
          DB_HOST: ${{ secrets.DB_HOST }}
          DB_USER: ${{ secrets.DB_USER }}
          PGPASSWORD: ${{ secrets.DB_PASSWORD }}
        run: |
          cd go_sprint
          ./scripts/backup-system.sh
      
      - name: Upload to S3
        uses: aws-actions/configure-aws-credentials@v1
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-east-1
      
      - name: Sync to S3
        run: |
          aws s3 sync backups/ s3://my-backups/sprint-management/ \
            --exclude "*" --include "system_backup_*.tar.gz"
```

## Monitoring and Alerts

### Backup Monitoring Script

```bash
#!/bin/bash
# Monitor backup health

BACKUP_DIR="./backups"
MAX_AGE_HOURS=25  # Alert if backup older than 25 hours

LATEST_BACKUP=$(ls -t "$BACKUP_DIR"/sprint_management_*.sql | head -1)

if [ -z "$LATEST_BACKUP" ]; then
    echo "ERROR: No backups found"
    exit 1
fi

BACKUP_AGE=$(($(date +%s) - $(stat -f %m "$LATEST_BACKUP")))
BACKUP_AGE_HOURS=$((BACKUP_AGE / 3600))

if [ $BACKUP_AGE_HOURS -gt $MAX_AGE_HOURS ]; then
    echo "WARNING: Last backup is $BACKUP_AGE_HOURS hours old"
    exit 1
fi

echo "OK: Last backup is $BACKUP_AGE_HOURS hours old"
```

## Additional Resources

- [Disaster Recovery Procedures](./disaster-recovery.md)
- [Database Setup Guide](./database-setup.md)
- [PostgreSQL Backup Documentation](https://www.postgresql.org/docs/current/backup.html)
- [NATS Configuration](./nats.md)

## Support

For backup and restore issues:
1. Check the troubleshooting section above
2. Review backup logs
3. Verify system requirements
4. Contact the database administrator
5. Refer to disaster recovery procedures
