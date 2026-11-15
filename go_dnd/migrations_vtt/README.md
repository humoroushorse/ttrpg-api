# go_dnd's own migrations: schema `dnd_vtt`

The tabletop's tables (scenes, tokens, the campaign event log). Only go_dnd uses them, so unlike
`../migrations` (a mirror of py_dnd's Alembic schema that is never applied to the shared database),
these **are** applied to the shared database, by go_dnd only:

- They live in schema `dnd_vtt`. Alembic's `env.py` reflects schema `dnd` only, so it never proposes
  dropping them.
- Their version table is `dnd_vtt.schema_migrations`, apart from Alembic's `dnd.alembic_version`.
  `cmd/migrate -schema dnd_vtt` creates the schema first (the version table needs it), and
  `000001`'s down migration leaves the schema in place for the same reason.
- They point at shared tables (`dnd.campaign`, `dnd.campaign_character`, `dnd."user"`) across schemas.
  Those foreign keys also stop an Alembic downgrade from dropping a shared table that the tabletop
  still uses.
- `dnd_vtt.event` is partitioned by month (`000003`). Its partitions are made and dropped by go_dnd's
  daily job through `dnd_vtt.create_event_partitions` and `dnd_vtt.drop_event_partitions`, not by
  migrations. `event_default` catches rows if the job falls behind and should stay empty.
- In production these run as a Kubernetes Job before the new pods start
  (`/app/migrate -schema dnd_vtt -path /app/migrations_vtt`). golang-migrate takes an advisory lock, so
  two runs at once can't both apply a migration.

**A change:**
1. `make vtt-migrate-create NAME=...`, then write the up and down SQL. Qualify every name with
   `dnd_vtt.` or `dnd.`.
2. `make vtt-migrate-up` (shared database `ttrpg-pg`; `DB_NAME=...` for another one).
   `make vtt-migrate-down` rolls back one migration, `make vtt-migrate-version` shows where it is.
3. `make sqlc-generate`. The `tabletop` package reads `../migrations` and this directory together
   (`sqlc.yaml`); its queries are in `internal/repository/tabletop/queries`.

Why: `DECISIONS.md` at the workspace root, "One database, two migration histories".
