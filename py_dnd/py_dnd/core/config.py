"""App configuration file."""

import logging
import os
from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict

DOTENV = os.path.join(os.path.dirname(__file__), "..", "..", ".env")


class Settings(BaseSettings):
    """Application environment variables Settings object."""

    API_DESCRIPTION: str | None = "Personal project for DnD"
    API_NAME: str | None = "Dungeons & Dragons FastAPI"
    API_VERSION: str | None = "0.1.0"
    API_V1_STR: str | None = "/api/v1"

    APP_HOST: str | None = "0.0.0.0"
    APP_PORT: int | None = 8001

    LOG_TO_FILE: bool | None = False
    LOG_LEVEL: str | int | None = logging.INFO
    LOG_RETENTION: str | int | None = "10 minutes"
    LOG_ROTATION: str | None = "1 minute"
    LOG_DIAGNOSE: bool | None = False  # DO NOT SET TO TRUE IN PROD

    REDOC_URL: str | None = "/redoc"
    SWAGGER_URL: str | None = "/docs"

    # SECRET_KEY: str = secrets.token_urlsafe(32)
    POSTGRES_MASTER_URI: str = os.getenv(
        "POSTGRES_MASTER_URI",
        f"postgresql+asyncpg://{os.getenv("POSTGRES_MASTER_USER", "postgres")}"
        + f":{os.getenv("POSTGRES_MASTER_PASSWORD", "admin")}"
        + f"@{os.getenv("POSTGRES_MASTER_HOST", "localhost")}"
        + f":{os.getenv("POSTGRES_MASTER_PORT", "5432")}"
        + f"/{os.getenv("POSTGRES_MASTER_NAME", "ttrpg-pg")}",
    )

    # default: postgresql+asyncpg://postgres:admin@localhost:5432/ttrpg-pg
    POSTGRES_REPLICA_URI: str = os.getenv(
        "POSTGRES_REPLICA_URI",
        f"postgresql+asyncpg://{os.getenv("POSTGRES_REPLICA_USER", "postgres")}"
        + f":{os.getenv("POSTGRES_REPLICA_PASSWORD", "admin")}"
        + f"@{os.getenv("POSTGRES_REPLICA_HOST", "localhost")}"
        + f":{os.getenv("POSTGRES_REPLICA_PORT", "5432")}"
        + f"/{os.getenv("POSTGRES_REPLICA_NAME", "ttrpg-pg")}",
    )

    model_config = SettingsConfigDict(
        env_file=DOTENV,
        extra="ignore",
    )

    # Database Pool Settings
    DB_POOL_SIZE: int = 20
    DB_MAX_OVERFLOW: int = 0  # Set to 0 for dev (breaks concurrent ops), 10-20 for prod
    DB_POOL_PRE_PING: bool = False  # Set to False for dev (breaks concurrent ops), True for prod
    DB_POOL_RECYCLE: int = 3600  # Recycle connections every hour
    DB_POOL_TIMEOUT: int = 30  # Wait time for connection from pool

    KEYCLOAK_SERVER_URL: str
    KEYCLOAK_REALM_NAME: str
    KEYCLOAK_CLIENT_ID: str
    KEYCLOAK_ADMIN_USERNAME: str
    KEYCLOAK_ADMIN_PASSWORD: str
    # KEYCLOAK_CLIENT_SECRET_KEY: str

@lru_cache
def get_settings() -> Settings:
    """Get applications settings (cached)."""
    return Settings()
