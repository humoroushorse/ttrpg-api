#!/usr/bin/env python3
"""Build go_dnd's OpenAPI specs from the per-feature files in api/openapi/src/.

- api/openapi/dnd.yaml: every feature file merged (the oapi-codegen input).
- api/openapi/combined-with-auth.yaml: dnd.yaml plus go_auth's endpoints (what /api/openapi.yaml serves).

Feature files hold `paths` and `components`; `00-base.yaml` also holds `openapi`, `info`, `tags`,
`servers` and `security`. A path or schema defined in two files is an error.
"""

import sys
from pathlib import Path

import yaml

script_dir = Path(__file__).parent.absolute()
src_dir = script_dir / "api/openapi/src"

WARNING = """# AUTO-GENERATED FILE - DO NOT EDIT. Edit api/openapi/src/*.yaml, then run `make merge-openapi`.
"""


def merge_into(target: dict, section: dict, where: str) -> None:
    for name, value in section.items():
        if name in target:
            sys.exit(f"{where}: '{name}' is defined twice")
        target[name] = value


dnd: dict = {"paths": {}, "components": {"schemas": {}, "responses": {}, "parameters": {}, "securitySchemes": {}}}
for path in sorted(src_dir.glob("*.yaml")):
    doc = yaml.safe_load(path.read_text()) or {}
    for key in ("openapi", "info", "servers", "security"):
        if key in doc:
            dnd[key] = doc[key]
    for tag in doc.get("tags") or []:  # every file's tags, once each
        if tag not in dnd.setdefault("tags", []):
            dnd["tags"].append(tag)
    merge_into(dnd["paths"], doc.get("paths") or {}, f"{path.name} paths")
    for section, items in (doc.get("components") or {}).items():
        merge_into(dnd["components"].setdefault(section, {}), items or {}, f"{path.name} components.{section}")
dnd["components"] = {k: v for k, v in dnd["components"].items() if v}
ordered = {k: dnd[k] for k in ("openapi", "info", "servers", "tags", "security", "paths", "components") if k in dnd}

with open(script_dir / "api/openapi/dnd.yaml", "w") as f:
    f.write(WARNING)
    yaml.dump(ordered, f, default_flow_style=False, sort_keys=False, allow_unicode=True, width=110)

with open(script_dir.parent / "go_auth/api/openapi/auth.yaml") as f:
    auth = yaml.safe_load(f)

combined = {
    "openapi": "3.0.3",
    "info": {
        "title": "DnD API with Authentication",
        "version": "1.0.0",
        "description": "Complete DnD API including authentication endpoints",
    },
    # relative, so the docs page calls whatever host served it (localhost, 127.0.0.1, a proxy):
    # same origin, no CORS
    "servers": [{"url": "/", "description": "This server"}],
    "tags": auth.get("tags", []) + ordered.get("tags", []),
    "paths": {"/api/v1" + path: methods for path, methods in auth.get("paths", {}).items()},
    "components": {"schemas": {}, "responses": {}, "parameters": {}, "securitySchemes": {}},
    "security": [{"BearerAuth": []}],
}
combined["paths"].update(ordered["paths"])
for spec in (ordered, auth):
    for section, items in (spec.get("components") or {}).items():
        combined["components"].setdefault(section, {}).update(items or {})
combined["components"] = {k: v for k, v in combined["components"].items() if v}

with open(script_dir / "api/openapi/combined-with-auth.yaml", "w") as f:
    f.write(WARNING)
    yaml.dump(combined, f, default_flow_style=False, sort_keys=False, allow_unicode=True, width=110)

print(f"Generated dnd.yaml ({len(ordered['paths'])} paths) and combined-with-auth.yaml ({len(combined['paths'])} paths)")
