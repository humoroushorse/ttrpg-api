"""FastAPI route definitions."""

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from py_dnd.core.config import get_settings
from py_dnd.features.auth.router import router as auth_router
from py_dnd.features.backgrounds.router import router as background_router
from py_dnd.features.classes.router import router as class_router
from py_dnd.features.conditions.router import router as condition_router
from py_dnd.features.core.router import router as core_router
from py_dnd.features.feats.router import router as feat_router
from py_dnd.features.glossary.router import router as glossary_router
from py_dnd.features.items.router import router as item_router
from py_dnd.features.sources.router import router as source_router
from py_dnd.features.species.router import router as species_router
from py_dnd.features.spells.router import router as spell_router
from py_dnd.features.spells_to_classes.router import router as spell_list_router
from py_dnd.features.subclasses.router import router as subclass_router
from py_dnd.features.user.router import router as user_router

# global route collection
api_router = APIRouter(default_response_class=JSONResponse)
# unprefixed for health checks
api_router.include_router(core_router, prefix="", tags=[])

public_routes = APIRouter()
public_routes.include_router(auth_router, prefix="/auth", tags=["Auth"])
public_routes.include_router(user_router, prefix="/users", tags=["User"])
public_routes.include_router(source_router, prefix="/sources", tags=["Source"])
public_routes.include_router(spell_router, prefix="/spells", tags=["Spell"])
public_routes.include_router(condition_router, prefix="/conditions", tags=["Condition"])
public_routes.include_router(glossary_router, prefix="/glossary", tags=["Glossary"])
public_routes.include_router(item_router, prefix="/items", tags=["Item"])
public_routes.include_router(feat_router, prefix="/feats", tags=["Feat"])
public_routes.include_router(species_router, prefix="/species", tags=["Species"])
public_routes.include_router(background_router, prefix="/backgrounds", tags=["Background"])
# before class_router: its /{source_slug}/{slug} would match /{class_id}/spells
public_routes.include_router(spell_list_router, prefix="/classes", tags=["Class Spell List"])
public_routes.include_router(class_router, prefix="/classes", tags=["Class"])
public_routes.include_router(subclass_router, prefix="/subclasses", tags=["Subclass"])

api_router.include_router(public_routes, prefix=get_settings().API_V1_STR or "")
