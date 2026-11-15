"""Alembic file for managing/running database migrations."""

import asyncio
import pathlib
import sys
from logging.config import fileConfig

from alembic import context
from sqlalchemy import create_engine, text
from sqlalchemy.engine import Connection
from sqlalchemy.ext.asyncio import AsyncEngine

# all models must be imported before attempting to define relationships
import py_dnd.database.all_models  # noqa: F401
from py_dnd.core.config import Settings, get_settings
from py_dnd.database.base_class import DndSchemaBase
from py_dnd.shared.enums import DbSchemaEnum

sys.path.append(str(pathlib.Path(__file__).resolve().parents[1]))

settings: Settings = get_settings()

# this is the Alembic Config object, which provides
# access to the values within the .ini file in use.
config = context.config

# Interpret the config file for Python logging.
# This line sets up loggers basically.
if config.config_file_name is not None:
    fileConfig(config.config_file_name)

# add your model's MetaData object here
# for 'autogenerate' support
# from myapp import mymodel
# target_metadata = mymodel.Base.metadata
target_metadata = DndSchemaBase.metadata

# Alembic owns schema `dnd` only. Other schemas in this database belong to other tools: go_dnd's
# Go-only tables (`dnd_vtt`), Keycloak (`keycloak_schema`), and `public`. Autogenerate must not see
# them, or it proposes dropping every table it has no model for.
MANAGED_SCHEMA = DbSchemaEnum.DND.value


def include_name(name: str | None, type_: str, parent_names: object) -> bool:  # noqa: ARG001
    """Reflect only the managed schema (``None`` is the connection's default schema, ``public``)."""
    if type_ == "schema":
        return name == MANAGED_SCHEMA
    return True


# the same options online and offline
CONTEXT_OPTS: dict[str, object] = {
    "target_metadata": target_metadata,
    "version_table_schema": MANAGED_SCHEMA,
    "include_schemas": True,
    "include_name": include_name,
    "compare_type": True,
}


def run_migrations_offline() -> None:
    """Run migrations in 'offline' mode.

    This configures the context with just a URL
    and not an Engine, though an Engine is acceptable
    here as well.  By skipping the Engine creation
    we don't even need a DBAPI to be available.

    Calls to context.execute() here emit the given string to the
    script output.

    """
    context.configure(
        url=settings.POSTGRES_MASTER_URI,
        literal_binds=True,
        dialect_opts={"paramstyle": "named"},
        **CONTEXT_OPTS,  # type: ignore[arg-type]
    )

    with context.begin_transaction():
        context.run_migrations()


def do_run_migrations(connection: Connection) -> None:
    """Run migrations with your given database connection."""
    context.configure(connection=connection, **CONTEXT_OPTS)  # type: ignore[arg-type]

    with context.begin_transaction():
        context.run_migrations()


async def run_migrations_online() -> None:
    """Run migrations in 'online' mode.

    In this scenario we need to create an Engine
    and associate a connection with the context.

    """
    # connectable = AsyncEngine(
    #     engine_from_config(
    #         config.get_section(config.config_ini_section),
    #         prefix="sqlalchemy.",
    #         poolclass=pool.NullPool,
    #         future=True,
    #     )
    # )
    connectable = AsyncEngine(
        create_engine(
            settings.POSTGRES_MASTER_URI,
            # echo=True,  # DEBUGGING
            future=True,
        )
    )

    async with connectable.connect() as connection:
        await connection.execute(text(f"CREATE SCHEMA IF NOT EXISTS {MANAGED_SCHEMA}"))
        await connection.commit()
        await connection.run_sync(do_run_migrations)

    await connectable.dispose()


if context.is_offline_mode():
    run_migrations_offline()
else:
    asyncio.run(run_migrations_online())
