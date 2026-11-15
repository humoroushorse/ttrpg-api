#!/usr/bin/env bash
# The `dnd` schema has two definitions: py_dnd's Alembic revisions (applied to the shared database)
# and go_dnd's golang-migrate mirror (read by sqlc, never applied to the shared database). They must
# build identical schemas (DECISIONS.md, "One database, two migration histories").
#
#   dnd_schema.sh dump [database]   normalized `pg_dump --schema-only` of schema dnd (default ttrpg-pg)
#   dnd_schema.sh check             build dnd with each tool in scratch databases and diff the dumps
#
# Runs pg_dump and psql inside the Postgres container, so the host needs neither.
set -euo pipefail

RUNNER="${CONTAINER_RUNNER:-docker}"
CONTAINER="${PG_CONTAINER:-ttrpg-postgresql}"
PG_USER="${PG_USER:-postgres}"
PG_PASSWORD="${PG_PASSWORD:-admin}"
PG_HOST="${PG_HOST:-localhost}"
PG_PORT="${PG_PORT:-5432}"
API_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ALEMBIC_DB=dnd_schema_check_alembic
GO_DB=dnd_schema_check_go

psql_c() {
  "$RUNNER" exec -e PGOPTIONS="-c client_min_messages=warning" "$CONTAINER" psql -U "$PG_USER" -v ON_ERROR_STOP=1 -qtc "$1" >/dev/null
}

# pg_dump without what differs run to run (the \restrict key) or doesn't belong in a migration
# (session SETs, comments, Alembic's version table), and with blank lines squeezed
dump() {
  "$RUNNER" exec "$CONTAINER" pg_dump -U "$PG_USER" -d "$1" --schema-only --schema=dnd \
    --no-owner --no-privileges --exclude-table=dnd.alembic_version |
    grep -v -E '^(\\restrict|\\unrestrict|SET |SELECT pg_catalog\.set_config|--)' |
    sed 's/^CREATE SCHEMA dnd;$/CREATE SCHEMA IF NOT EXISTS dnd;/' |
    sed '/./,$!d' |
    cat -s
}

drop_scratch() {
  psql_c "DROP DATABASE IF EXISTS $ALEMBIC_DB WITH (FORCE)"
  psql_c "DROP DATABASE IF EXISTS $GO_DB WITH (FORCE)"
}

check() {
  trap drop_scratch EXIT
  drop_scratch
  psql_c "CREATE DATABASE $ALEMBIC_DB"
  psql_c "CREATE DATABASE $GO_DB"
  (
    cd "$API_DIR/py_dnd"
    POSTGRES_MASTER_URI="postgresql+asyncpg://$PG_USER:$PG_PASSWORD@$PG_HOST:$PG_PORT/$ALEMBIC_DB" \
      poetry run alembic upgrade head >/dev/null 2>&1
  )
  migrate -path "$API_DIR/go_dnd/migrations" \
    -database "postgresql://$PG_USER:$PG_PASSWORD@$PG_HOST:$PG_PORT/$GO_DB?sslmode=disable" up >/dev/null 2>&1
  if diff -u --label alembic <(dump "$ALEMBIC_DB") --label go_dnd <(dump "$GO_DB"); then
    echo "dnd schema: Alembic and go_dnd's migrations match"
  else
    echo "dnd schema differs (- Alembic, + go_dnd). Add the missing migration on the + or - side." >&2
    exit 1
  fi
}

case "${1:-}" in
  dump) dump "${2:-ttrpg-pg}" ;;
  check) check ;;
  *) echo "usage: $0 dump [database] | check" >&2; exit 2 ;;
esac
