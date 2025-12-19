"""Spell schemas."""
import re
import uuid
from uuid6 import uuid7
from typing import Any

from pydantic import AliasChoices, BaseModel, ConfigDict, Field, StrictInt

from py_dnd.shared import enums as shared_enums
from py_dnd.shared.schemas import MixinBookeeping, MixinBookeepingCreate, MixinBookeepingUpdate, QueryBase, MixinUuid7Id


class SpellSchema(MixinBookeeping, MixinUuid7Id):
    """How the spell shows up in the database."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
    )

    source_id: uuid.UUID = Field(
        title="Source ID",
        description="[Generated] Unique database ID For the Source table.",
    )
    name: str = Field(title="Name", description="The name of the spell.")
    # dnd_version: str = Field(title="D&D Version", description="The version of Dungeons and Dragons.")
    # dnd_version_year: int = Field(
    #     title="D&D Version Year", description="The year that the version of Dungeons and Dragons came out."
    # )
    source_page: StrictInt | None = Field(
        default=None,
        title="Source Page Number",
        description="The page number of the source material tha the spell can be found in.",
    )
    level: shared_enums.SpellLevelEnum = Field(
        title="Level", description="The minimum slot level required to cast the spell."
    )
    school: shared_enums.SpellSchoolEnum = Field(title="School", description="The school of magic of the spell.")
    is_ritual: bool = Field(title="Is Ritual", description="Can cast the spell ritually?")
    casting_time: str = Field(title="Casting Time", description="How long it takes to cast the spell")
    range: str = Field(title="Range", description="How range of the spell")
    has_verbal_component: bool = Field(
        title="Has Verbal Component", description="Does the spell have a verbal (V) component?"
    )
    has_somatic_component: bool = Field(
        title="Has Somatic Component", description="Does the spell have a somatic (S) component?"
    )
    has_material_component: bool = Field(
        title="Has Material Component", description="Does the spell have a material (M) component?"
    )
    materials: str | None = Field(
        default=None, title="Materials", description="The materials required to cast the spell."
    )
    has_spell_cost: bool = Field(default=False, description="[Generated] is there some cost?")
    are_materials_consumed: bool = Field(
        title="Materials",
        description="[Generated] Are (at least some of) the materials consumed when you cast the spell?",
    )
    duration: str = Field(title="Duration", description="How long the spell lasts.")
    is_concentration: bool = Field(
        title="Is Concentration", description="[Generated] Does the spell require concentration to maintain it?"
    )
    description: str = Field(
        title="Description",
        description="Spell Description",
        # description_html in the source data (can use markdown instead)
        validation_alias=AliasChoices("description", "description_html", "descriptionHtml"),
    )
    has_saving_throw: bool = Field(
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
    difficulty_class_saving_throw: str | StrictInt | None = Field(
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


class SpellCreateRaw(BaseModel):
    """Raw spell schema for parsing the source data."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
    )

    components: str = Field(
        title="Components",
        description="The components required to cast the spell. Can be a combination of V (verbal), S (Somatic), and M (Material).",
    )


class SpellCreate(BaseModel):
    """Allowed fields for creating a spell."""

    model_config = ConfigDict(extra="ignore", validate_assignment=True)

    source_id: uuid.UUID = Field(
        title="Source ID",
        description="[Generated] Unique database ID For the Source table.",
    )
    name: str = Field(title="Name", description="The name of the spell.")
    source_page: StrictInt | None = Field(
        default=None,
        title="Source Page Number",
        description="The page number of the source material tha the spell can be found in.",
    )
    level: shared_enums.SpellLevelEnum = Field(
        title="Level", description="The minimum slot level required to cast the spell."
    )
    school: shared_enums.SpellSchoolEnum = Field(title="School", description="The school of magic of the spell.")
    is_ritual: bool = Field(title="Is Ritual", description="Can cast the spell ritually?")
    casting_time: str = Field(title="Casting Time", description="How long it takes to cast the spell")
    range: str = Field(title="Range", description="How range of the spell")
    has_verbal_component: bool = Field(
        title="Has Verbal Component", description="Does the spell have a verbal (V) component?"
    )
    has_somatic_component: bool = Field(
        title="Has Somatic Component", description="Does the spell have a somatic (S) component?"
    )
    has_material_component: bool = Field(
        title="Has Material Component", description="Does the spell have a material (M) component?"
    )
    materials: str | None = Field(
        default=None, title="Materials", description="The materials required to cast the spell."
    )
    has_spell_cost: bool = Field(default=False, description="[Generated] is there some cost?")
    are_materials_consumed: bool = Field(
        title="Materials",
        description="[Generated] Are (at least some of) the materials consumed when you cast the spell?",
    )
    duration: str = Field(title="Duration", description="How long the spell lasts.")
    is_concentration: bool = Field(
        title="Is Concentration", description="[Generated] Does the spell require concentration to maintain it?"
    )
    description: str = Field(
        title="Description",
        description="Spell Description",
        # description_html in the source data (can use markdown instead)
        validation_alias=AliasChoices("description", "description_html", "descriptionHtml"),
    )
    has_saving_throw: bool = Field(
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
    difficulty_class_saving_throw: str | StrictInt | None = Field(
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


    # @classmethod
    # def from_raw(cls, raw: SpellCreateRaw) -> 'SpellCreate':
    #     text = raw.components.strip()

    #     # Extract the part in parentheses (if present)
    #     match = re.match(r'^(.*?)\s*(?:\((.*?)\))?$', text)
    #     component_part = match.group(1) if match else ''
    #     material_details = match.group(2) if match else None

    #     # Parse component letters
    #     components = [c.strip() for c in component_part.split(',')]
    #     has_verbal = 'V' in components
    #     has_somatic = 'S' in components
    #     has_material = 'M' in components

    #     # Detect cost and consumption
    #     has_cost = False
    #     is_consumed = False

    #     if material_details:
    #         # Simplified detection logic
    #         if re.search(r'\bworth\b|\bgp\b|\bgold\b|\bcost\b', material_details, re.IGNORECASE):
    #             has_cost = True
    #         if re.search(r'\bconsume[sd]?\b|\bconsumed\b|\bconsumes\b', material_details, re.IGNORECASE):
    #             is_consumed = True

    #     return cls(
    #         has_verbal_component=has_verbal,
    #         has_somatic_component=has_somatic,
    #         has_material_component=has_material,
    #         materials=material_details,
    #         has_spell_cost=has_cost,
    #         are_materials_consumed=is_consumed
    #     )


class SpellCreateDerrived(SpellCreate, MixinBookeepingCreate, MixinUuid7Id):
    """Add derrived fields for creating a spell."""
    pass


class SpellUpdate(BaseModel):
    """Allowed fields for editing a spell."""
    # TODO: allowed fields for editing a spell
    pass


class SpellUpdateDerrived(SpellUpdate, MixinBookeepingUpdate):
    """Allowed fields for editing a spell."""
    pass


class SpellQuery(QueryBase):
    """Allowed fields for querying spells."""

    name: str | None = Field(default=None, title="Names", description='Name(s) to filter on (separated by commas ",")')
