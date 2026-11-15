"""Load every ETL file (2014 and 2024) into the local database through the bulk routes.

Temporary and local only: it runs the app in-process with auth switched off, so no server or token
is needed, and audit columns (created_by, ...) stay null. Run it after the tables exist:

    make obliterate-migration   # or make alembic-upgrade on an empty database
    make load-seed-data

Set DND_DATA_DIR to read the ETL output from somewhere other than ttrpg-data-dnd next to ttrpg-api.
"""

# ruff: noqa: T201  # a command-line script: printing the report is its output

import json
import os
import pathlib
import sys

from fastapi.testclient import TestClient

from py_dnd.core.config import get_settings
from py_dnd.features.auth.schemas import AuthUserToken
from py_dnd.features.auth.service import get_auth
from py_dnd.features.user.service import get_current_user_id
from py_dnd.main import app

DATA_DIR = pathlib.Path(
    os.environ.get(
        "DND_DATA_DIR",
        pathlib.Path(__file__).resolve().parents[3] / "ttrpg-data-dnd" / "playwright-data" / "data" / "processed",
    )
)
YEARS = ("2014", "2024")

# (route, file under DATA_DIR) in load order: sources, then spells, then the rest, then spell lists
LOADS = [
    *[("/sources/bulk", f"sources/sources_{year}.json") for year in YEARS],
    *[
        (f"/{route}/bulk", f"{folder}/{year}/_all.json")
        for route, folder in [
            ("spells", "spells"),
            ("conditions", "conditions"),
            ("glossary", "glossary"),
            ("items", "items"),
            ("feats", "feats"),
            ("species", "species"),
            ("backgrounds", "backgrounds"),
            ("classes", "classes"),
            ("subclasses", "subclasses"),
        ]
        for year in YEARS
    ],
    *[("/classes/spell-lists/bulk", f"spells/{year}/_all.json") for year in YEARS],
]

SHOW = 5  # errors / warnings printed per file


def main() -> int:
    """Load each file in order and print its totals. Exit 1 if any file had errors."""
    app.dependency_overrides[get_auth] = lambda: AuthUserToken.model_construct(
        sub="load-seeds", preferred_username="load-seeds"
    )
    app.dependency_overrides[get_current_user_id] = lambda: None
    # the app's startup only runs once per process
    with TestClient(app) as client:
        return 0 if load(client) else 1


def load(client: TestClient) -> bool:
    """POST each file to its bulk route and print the report. False if any file had errors."""
    prefix = get_settings().API_V1_STR or ""
    failed = False
    for route, name in LOADS:
        path = DATA_DIR / name
        if not path.exists():
            print(f"SKIP  {route:28} {name}: no file")
            continue
        if not json.loads(path.read_text()):
            print(f"SKIP  {route:28} {name}: empty")
            continue
        response = client.post(f"{prefix}{route}", files={"file": (path.name, path.read_bytes(), "application/json")})
        if response.status_code != 200:
            failed = True
            print(f"FAIL  {route:28} {name}: HTTP {response.status_code} {response.text[:300]}")
            continue
        report = response.json()
        errors, warnings = report.get("errors", []), report.get("warnings", [])
        failed = failed or bool(errors)
        totals = ", ".join(f"{k} {v}" for k, v in report["totals"].items())
        print(f"{'ERROR' if errors else 'OK':5} {route:28} {name}: {totals}")
        for line in errors[:SHOW]:
            print(f"        error: {line}")
        for line in warnings[:SHOW]:
            print(f"        warning: {line}")
        if len(errors) > SHOW or len(warnings) > SHOW:
            print(f"        ... {len(errors)} errors, {len(warnings)} warnings in all")
    return not failed


if __name__ == "__main__":
    sys.exit(main())
