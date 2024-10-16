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
    APP_PORT: int | None = 8002

    LOG_TO_FILE: bool | None = False
    LOG_LEVEL: str | int | None = logging.INFO
    LOG_RETENTION: str | int | None = "10 minutes"
    LOG_ROTATION: str | None = "1 minute"
    LOG_DIAGNOSE: bool | None = False  # DO NOT SET TO TRUE IN PROD

    REDOC_URL: str | None = "/redoc"
    SWAGGER_URL: str | None = "/docs"

    # SECRET_KEY: str = secrets.token_urlsafe(32)
    POSTGRES_DATABASE_URI: str = os.getenv(
        "POSTGRES_DATABASE_URI",
        f"postgresql+asyncpg://{os.getenv("POSTGRES_USER", "postgres")}:{os.getenv("POSTGRES_PASSWORD", "admin")}"
        + f"@{os.getenv("POSTGRES_HOST", "localhost")}:{os.getenv("POSTGRES_PORT", "5432")}"
        + f"/{os.getenv("POSTGRES_NAME", "ttrpg-pg")}",
    )

    model_config = SettingsConfigDict(
        env_file=DOTENV,
        extra="ignore",
    )

    KEYCLOAK_SERVER_URL: str
    KEYCLOAK_REALM_NAME: str
    KEYCLOAK_CLIENT_ID: str
    # KEYCLOAK_CLIENT_SECRET_KEY: str


@lru_cache
def get_settings() -> Settings:
    """Get applications settings (cached)."""
    return Settings()
