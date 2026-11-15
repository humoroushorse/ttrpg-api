"""Bulk upload files: read a JSON or CSV upload into plain rows."""

import json
import math
from io import BytesIO
from typing import Any

from fastapi import HTTPException, UploadFile, status
lazy import pandas as pd

ALLOWED_TYPES = ("application/json", "text/csv")


def _clean(value: Any) -> Any:
    """CSV cells: NaN is empty, and pandas reads integer columns with gaps as floats (112.0)."""
    if isinstance(value, float):
        if math.isnan(value):
            return None
        if value.is_integer():
            return int(value)
    return value


async def read_rows(file: UploadFile) -> list[tuple[int, dict[str, Any]]]:
    """(row number, row) for each record in a JSON or CSV upload.

    JSON must be a list of objects and is kept as parsed, so nested values (a spell's ``vtt``) stay
    objects. CSV goes through pandas.
    """
    contents = await file.read()
    if file.content_type == "application/json":
        try:
            data = json.loads(contents)
        except json.JSONDecodeError as e:
            raise HTTPException(status.HTTP_400_BAD_REQUEST, f"Invalid JSON: {e}") from e
        if not isinstance(data, list) or not all(isinstance(row, dict) for row in data):
            raise HTTPException(status.HTTP_400_BAD_REQUEST, "Expected a JSON list of objects.")
        return list(enumerate(data))
    if file.content_type == "text/csv":
        df = pd.read_csv(BytesIO(contents))
        return [(i, {k: _clean(v) for k, v in row.items()}) for i, row in enumerate(df.to_dict("records"))]
    raise HTTPException(
        status.HTTP_400_BAD_REQUEST,
        f"Invalid document type. Expected one of: {list(ALLOWED_TYPES)}, Received: {file.content_type}",
    )
