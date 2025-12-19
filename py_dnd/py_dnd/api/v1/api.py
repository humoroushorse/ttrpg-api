"""FastAPI route definitions."""

from fastapi import APIRouter


from py_dnd.features.auth.router import router as auth_router
from py_dnd.features.core.router import router as core_router
from py_dnd.features.sources.router import router as source_router
from py_dnd.features.spells.router import router as spell_router

api_router = APIRouter()

public_routes = APIRouter()
public_routes.include_router(core_router, prefix="", tags=[])
public_routes.include_router(auth_router, prefix="/auth", tags=["Auth"])
public_routes.include_router(spell_router, prefix="/spells", tags=["Spells"])
public_routes.include_router(source_router, prefix="/sources", tags=["Sources"])

api_router.include_router(public_routes)