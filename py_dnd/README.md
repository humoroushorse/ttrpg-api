# py_dnd

## VTT spell models

`py_dnd/features/spells/vtt_models.py` holds the Pydantic models for a spell's `vtt` block. The
file is generated; don't edit it.

| What | Where |
|---|---|
| Source of truth (the ETL's models) | `ttrpg-data-dnd/playwright-data/transformer/playwright_data_transformer/vtt/` |
| JSON Schema the ETL writes (`make vtt_schema` there) | `ttrpg-data-dnd/playwright-data/schemas/vtt_spell.v1.schema.json` |
| Copy shared by py_dnd and go_dnd | `ttrpg-api/schemas/vtt_spell.v1.schema.json` |
| Generator options | `pyproject.toml`, `[tool.datamodel-codegen.profiles.vtt_spell]` |

When to run:
- `make vtt-models` after the ETL schema changes. It copies the schema and regenerates the
  models. Commit both files.
- `make vtt-models-check` before pushing. It fails if the copy differs from the ETL's file, or if
  the models differ from what the copy generates. The ETL repo is expected at
  `../../ttrpg-data-dnd` (override with `VTT_SCHEMA_SRC=<path>`); without it, only the models
  are checked against the copy.

The models reject unknown keys. A spell file with a field added after the last `make vtt-models`
fails to load until you run it.
