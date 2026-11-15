"""A sqlalchemy.orm Session."""

from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from dnd.core import uncached_settings

engine = create_engine(uncached_settings.POSTGRES_DATABASE_URI, pool_pre_ping=True)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
