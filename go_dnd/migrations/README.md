# go_dnd migrations: a mirror of py_dnd's Alembic schema

py_dnd and go_dnd share one database and toggle on the same data. The shared tables (schema `dnd`)
must be identical in both, so they are defined twice:

- **py_dnd's Alembic revisions** are the original, and Alembic is the only tool that applies them to
  the shared database.
- **These files** mirror them. sqlc reads them (`sqlc.yaml`: `schema: "migrations"`), and they build
  scratch and test databases. **Never run them against the shared database:** the tables already exist
  there, and golang-migrate would start a second version history.

**A schema change:**
1. Change the SQLAlchemy model, then `make migration name="..."` in py_dnd, and apply it with
   `make alembic-upgrade`.
2. Add the same change here: `make migrate-create NAME=...`, then write the SQL.
3. `make schema-check`: builds `dnd` with each tool in scratch databases and diffs `pg_dump` of both.
   It must pass.
4. `make sqlc-generate`.

`000001_init` came from `scripts/dnd_schema.sh dump` of the database Alembic built at revision
`42fc01dee024`. Later files are hand-written from their Alembic revision (`000002` is `31ef9eb3cb82`).

**Go-only tables** (the VTT) live in schema `dnd_vtt`, in `../migrations_vtt`, which go_dnd does
apply, with its own version table. See the README there.

Why: `DECISIONS.md` at the workspace root, "One database, two migration histories".
