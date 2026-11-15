"""Shared pieces for catalog entities (conditions, glossary entries, and the ones after them).

A catalog entity is a book entry the ETL produces: keyed on (source, slug), loaded by bulk
upload, readable by id and by source + slug, with full CRUD. Spells came first and spell their
columns and routes out; new entities use ``MixinCatalogEntry``, ``CatalogFields`` and
``catalog_router``.
"""
