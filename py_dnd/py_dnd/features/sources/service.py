"""Source lookups the bulk uploads share."""

from typing import Any

from py_dnd.features.sources.repository import SourceRepository
from py_dnd.features.sources.schemas import SourceSchema


async def find_source_for_row(source_repo: SourceRepository, row: dict[str, Any]) -> SourceSchema | None:
    """An ETL row's source: by slug (the ETL's ``source_id``), then name + version, then short name."""
    source_name = row.get("source_name")
    source_short = row.get("source_id")
    if isinstance(source_short, str):
        sources, _ = await source_repo.query(params={"slug": source_short}, limit=1, exact=True)
        if sources:
            return sources[0]
    if source_name and row.get("dnd_version") and row.get("dnd_version_year"):
        sources, _ = await source_repo.query(
            params={
                "name": source_name,
                "dnd_version": row["dnd_version"],
                "dnd_version_year": int(row["dnd_version_year"]),
            },
            limit=1,
            exact=True,
        )
        if sources:
            return sources[0]
    if isinstance(source_short, str):
        for short in (source_short, source_short.upper()):
            sources, _ = await source_repo.query(params={"name_short": short}, limit=1)
            if sources:
                return sources[0]
    return None
