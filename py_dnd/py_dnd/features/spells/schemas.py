"""Spell schemas.

``SpellFields`` holds every field a client writes, once. The request bodies build on it:
``SpellCreateInput`` (POST) and ``SpellReplaceInput`` (PUT) need the whole spell,
``SpellPatchInput`` (PATCH) takes any subset. ``SpellSchema`` is what the API returns, with the
database-generated columns. Request bodies reject unknown keys; the bulk upload ignores them,
since the ETL's spell files carry extra keys (``as_html``, ``classes``, ...).
"""

import json
import uuid
from typing import TYPE_CHECKING, Any, Self

from pydantic import (
    AliasChoices,
    BaseModel,
    ConfigDict,
    Field,
    StrictInt,
    field_serializer,
    field_validator,
    model_validator,
)

from py_dnd.features.sources.schemas import SourceRef
from py_dnd.features.spells.vtt_models import VttSpellFields
from py_dnd.shared import enums as shared_enums
from py_dnd.shared.fields import fields_query, partial_model
from py_dnd.shared.schemas import (
    MixinBookeepingCreate,
    MixinBookeepingUpdate,
    QueryBase,
)

VTT_DESCRIPTION = "What the VTT needs to resolve the spell (schema ttrpg-api/schemas/vtt_spell.v1.schema.json)."


def _vtt_from_text(value: Any) -> Any:
    """A CSV upload carries the VTT block as JSON text."""
    return json.loads(value) if isinstance(value, str) else value


def _vtt_dump(vtt: VttSpellFields | None) -> dict[str, Any] | None:
    """Store and return the schema's key names ("from", not "from_")."""
    return vtt.model_dump(mode="json", by_alias=True) if vtt is not None else None


class MixinVttSpell(BaseModel):
    """The VTT block, for any system's spell schema (VTT_ROADMAP 4.2)."""

    vtt: VttSpellFields = Field(title="VTT", description=VTT_DESCRIPTION)

    _vtt_text = field_validator("vtt", mode="before")(_vtt_from_text)

    @field_serializer("vtt")
    def _vtt_by_alias(self, vtt: VttSpellFields) -> dict[str, Any] | None:
        return _vtt_dump(vtt)


class MixinVttSpellColumns(BaseModel):
    """Columns the database generates from ``vtt`` (read only; they describe the cast)."""

    activation_cost: str | None = Field(default=None, title="Activation Cost")
    activation_amount: int | None = Field(default=None, title="Activation Amount")
    range_kind: str | None = Field(default=None, title="Range Kind")
    range_feet: int | None = Field(default=None, title="Range (feet)")
    duration_kind: str | None = Field(default=None, title="Duration Kind")
    duration_amount: int | None = Field(default=None, title="Duration Amount")
    duration_unit: str | None = Field(default=None, title="Duration Unit")
    sustain: str | None = Field(default=None, title="Sustain", description="concentration (D&D)")
    has_attack_roll: bool | None = Field(default=None, title="Has Attack Roll")
    has_save: bool | None = Field(default=None, title="Has Save")
    has_summon: bool | None = Field(default=None, title="Has Summon")


def _dc_as_text(value: Any) -> Any:
    """The column is text; the ETL writes a number for a fixed DC."""
    return str(value) if isinstance(value, int) and not isinstance(value, bool) else value


class SpellFields(MixinVttSpell):
    """Every field a client writes."""

    model_config = ConfigDict(extra="forbid", validate_assignment=True, str_strip_whitespace=True)

    source_id: uuid.UUID = Field(
        title="Source ID",
        description="[Generated] Unique database ID For the Source table.",
    )
    slug: str = Field(title="Slug", description="Unique within its source (the natural key with source_id).")
    name: str = Field(title="Name", description="The name of the spell.")
    dnd_version: str = Field(title="D&D Version", description="The version of Dungeons and Dragons.")
    dnd_version_year: int = Field(
        title="D&D Version Year", description="The year that the version of Dungeons and Dragons came out."
    )
    source_page: StrictInt | None = Field(
        default=None,
        title="Source Page Number",
        description="The page number of the source material tha the spell can be found in.",
    )
    level: shared_enums.SpellLevelEnum = Field(
        title="Level", description="The minimum slot level required to cast the spell."
    )
    school: shared_enums.SpellSchoolEnum = Field(title="School", description="The school of magic of the spell.")
    is_ritual: bool = Field(default=False, title="Is Ritual", description="Can cast the spell ritually?")
    casting_time: str = Field(title="Casting Time", description="How long it takes to cast the spell")
    range: str = Field(title="Range", description="How range of the spell")
    has_verbal_component: bool = Field(
        default=False, title="Has Verbal Component", description="Does the spell have a verbal (V) component?"
    )
    has_somatic_component: bool = Field(
        default=False, title="Has Somatic Component", description="Does the spell have a somatic (S) component?"
    )
    has_material_component: bool = Field(
        default=False, title="Has Material Component", description="Does the spell have a material (M) component?"
    )
    materials: str | None = Field(
        default=None, title="Materials", description="The materials required to cast the spell."
    )
    has_spell_cost: bool = Field(default=False, description="[Generated] is there some cost?")
    are_materials_consumed: bool = Field(
        default=False,
        title="Materials",
        description="[Generated] Are (at least some of) the materials consumed when you cast the spell?",
    )
    duration: str = Field(title="Duration", description="How long the spell lasts.")
    is_concentration: bool = Field(
        default=False,
        title="Is Concentration",
        description="[Generated] Does the spell require concentration to maintain it?",
    )
    description: str = Field(
        title="Description",
        description="Spell Description",
        # description_html in the source data (can use markdown instead)
        validation_alias=AliasChoices("description", "description_html", "descriptionHtml"),
    )
    has_saving_throw: bool = Field(
        default=False,
        title="Has Saving Throw",
        description="[Generated] Does the spell cause an opponent to make a saving throw in any capacity?",
    )
    difficulty_class_saving_throw_override: StrictInt | None = Field(
        default=None,
        title="Difficulty Class (DC) Override",
        description="Override for the saving throw Difficulty Class (DC).",
    )
    # string is made up of DamageTypeEnum
    # TODO: actual list for the enum
    damage_type: str | None = Field(
        default=None,
        title="Damage Type",
        description="[Soon To Be Generated] Damage types that the spell can inflict.",
    )
    at_higher_levels: str | None = Field(
        default=None, title="At Higher Levels", description="How the spell changes when cast at higher levels."
    )
    difficulty_class_saving_throw: str | None = Field(
        default=None,
        title="Difficulty Class Saving Throw",
        description="If the spell has a saving throw element to it, what is the Difficulty Class (DC) of that save?",
    )
    # string is made up of AbilityScoreEnum
    # TODO: actual list for the enum
    difficulty_class_type: str | None = Field(
        default=None,
        title="Difficulty Class Saving Throw",
        description="What ability score is ued to make a save mentioned in the spell?",
    )
    stat_blocks: list[dict[str, Any]] | None = Field(
        default_factory=list,
        title="Stat Blocks",
        description="[TODO implement] The stat blocks of any creatures created/summoned/etc by this spell.",
    )
    is_unearthed_arcana: bool = Field(default=False, title="Is Unearthed Arcana")
    is_homebrew: bool = Field(default=False, title="Is Homebrew")

    _dc_text = field_validator("difficulty_class_saving_throw", mode="before")(_dc_as_text)


# Writable columns, for updates and the bulk upload's change check
SPELL_FIELDS = frozenset(SpellFields.model_fields)


class SpellSchemaBase(SpellFields, MixinBookeepingCreate, MixinBookeepingUpdate):
    """How the spell shows up in the database."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
        str_strip_whitespace=True,
    )

    id: uuid.UUID = Field(
        default_factory=uuid.uuid7,
        title="Spell ID",
        description="[Generated] Unique database ID For the Spell table.",
        validation_alias=AliasChoices("id", "spell_id", "spellId"),
    )


class SpellSchema(MixinVttSpellColumns, SpellSchemaBase):
    """How the spell shows up in the database (with relationships)."""

    source: SourceRef = Field(title="Source", description="[Read only] The book the spell is printed in.")


# What the read routes return: SpellSchema's fields, all optional, so ?fields= can leave some out.
# mypy sees SpellSchema (the generated class has no static type).
if TYPE_CHECKING:
    SpellSchemaPartial = SpellSchema
else:
    SpellSchemaPartial = partial_model(SpellSchema)
SPELL_READ_FIELDS = fields_query(SpellSchema)


class SpellCreateInput(SpellFields):
    """POST body: the whole spell. Unknown keys are errors."""


class SpellReplaceInput(SpellFields):
    """PUT body: the whole spell. Fields left out go back to their defaults."""


class SpellPatchInput(BaseModel):
    """PATCH body: only the fields to change. ``vtt`` is replaced whole, not merged."""

    model_config = ConfigDict(extra="forbid", validate_assignment=True, str_strip_whitespace=True)

    source_id: uuid.UUID | None = None
    slug: str | None = None
    name: str | None = None
    dnd_version: str | None = None
    dnd_version_year: int | None = None
    source_page: StrictInt | None = None
    level: shared_enums.SpellLevelEnum | None = None
    school: shared_enums.SpellSchoolEnum | None = None
    is_ritual: bool | None = None
    casting_time: str | None = None
    range: str | None = None
    has_verbal_component: bool | None = None
    has_somatic_component: bool | None = None
    has_material_component: bool | None = None
    materials: str | None = None
    has_spell_cost: bool | None = None
    are_materials_consumed: bool | None = None
    duration: str | None = None
    is_concentration: bool | None = None
    description: str | None = None
    has_saving_throw: bool | None = None
    difficulty_class_saving_throw_override: StrictInt | None = None
    damage_type: str | None = None
    at_higher_levels: str | None = None
    difficulty_class_saving_throw: str | None = None
    difficulty_class_type: str | None = None
    stat_blocks: list[dict[str, Any]] | None = None
    is_unearthed_arcana: bool | None = None
    is_homebrew: bool | None = None
    vtt: VttSpellFields | None = Field(default=None, title="VTT", description=VTT_DESCRIPTION)

    _vtt_text = field_validator("vtt", mode="before")(_vtt_from_text)
    _dc_text = field_validator("difficulty_class_saving_throw", mode="before")(_dc_as_text)

    @field_serializer("vtt")
    def _vtt_by_alias(self, vtt: VttSpellFields | None) -> dict[str, Any] | None:
        return _vtt_dump(vtt)

    @model_validator(mode="after")
    def _no_null_for_required(self) -> Self:
        """``null`` clears an optional field; a field the spell must have can't be cleared."""
        cleared = sorted(k for k in self.model_fields_set & _NOT_NULL if getattr(self, k) is None)
        if cleared:
            raise ValueError(f"these fields can't be null: {', '.join(cleared)}")
        return self

    def changes(self) -> dict[str, Any]:
        """The fields the request sets, ready for the repository."""
        return self.model_dump(exclude_unset=True)


# Fields that can't be null: no default, or a non-None default (the booleans)
_NOT_NULL = frozenset(
    name
    for name, info in SpellFields.model_fields.items()
    if info.is_required() or (info.default is not None and info.default_factory is None)
)
if set(SpellPatchInput.model_fields) != SPELL_FIELDS:  # keep PATCH in step with SpellFields
    raise RuntimeError("SpellPatchInput's fields differ from SpellFields'")


class SpellCreate(SpellSchemaBase, MixinBookeepingCreate, MixinBookeepingUpdate):
    """A new spell row (POST and the bulk upload). Ignores keys it doesn't know."""

    model_config = ConfigDict(extra="ignore", validate_assignment=True, str_strip_whitespace=True)


class SpellUpdate(SpellFields, MixinBookeepingUpdate):
    """The columns an update writes (typing for the repository)."""


class SpellQuery(QueryBase):
    """Allowed fields for querying spells."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        str_strip_whitespace=True,
    )

    name: str | None = Field(default=None, title="Names", description='Name(s) to filter on (separated by commas ",")')
    versions_of: str | None = Field(
        default=None,
        title="Versions Of",
        description="A slug: every printing of that spell across sources and editions. Hyphens are ignored, "
        "so bigbys-hand (2014) also finds bigby-s-hand (2024).",
    )
