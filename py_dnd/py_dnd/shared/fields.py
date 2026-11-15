"""Sparse fieldsets: ``?fields=name,level,school`` returns only those fields (and ``id``).

A whitelist, for bandwidth, not access control: leave ``fields`` off to get every field. The
database reads only the requested columns, and the response leaves the rest out (not ``null``).
"""

from collections.abc import Callable

from fastapi import HTTPException, Query, status
from pydantic import BaseModel, ConfigDict, Field, create_model

FIELDS_DESCRIPTION = "Comma-separated fields to return (id is always included). Leave it off for every field."


_PARTIALS: dict[type[BaseModel], type[BaseModel]] = {}


def partial_model(model: type[BaseModel]) -> type[BaseModel]:
    """The same fields, all optional: a sparse response sets only the ones asked for.

    Routes return it with ``response_model_exclude_unset=True``, so missing fields are left out.
    One class per model, so the routes and the repository share it (and OpenAPI names it once).
    """
    if model not in _PARTIALS:
        fields = {
            name: (info.annotation | None, Field(default=None, title=info.title, description=info.description))
            for name, info in model.model_fields.items()
        }
        _PARTIALS[model] = create_model(  # type: ignore[call-overload]
            f"{model.__name__}Partial", __config__=ConfigDict(from_attributes=True), **fields
        )
    return _PARTIALS[model]


def fields_query(model: type[BaseModel]) -> Callable[[str | None], tuple[str, ...]]:
    """A FastAPI dependency that reads ``?fields=`` against ``model``'s fields (422 on unknown)."""
    allowed = tuple(model.model_fields)

    def dependency(fields: str | None = Query(default=None, description=FIELDS_DESCRIPTION)) -> tuple[str, ...]:
        if not fields:
            return allowed
        names = list(dict.fromkeys(f.strip() for f in fields.split(",") if f.strip()))
        unknown = sorted(set(names) - set(allowed))
        if unknown:
            raise HTTPException(
                status.HTTP_422_UNPROCESSABLE_CONTENT,
                f"Unknown field(s): {', '.join(unknown)}. Allowed: {', '.join(sorted(allowed))}",
            )
        return ("id", *(n for n in names if n != "id"))

    return dependency
