"""Shared enums."""

from enum import Enum, IntEnum, StrEnum


class DbSchemaEnum(StrEnum):
    """Database Schema Options."""

    DND = "dnd"


class SpellSchoolEnum(StrEnum):
    """Spell school options."""

    ABJURATION = "abjuration"
    ALTERATION = "alteration"
    CONJURATION = "conjuration"
    DIVINATION = "divination"
    ENCHANTMENT = "enchantment"
    EVOCATION = "evocation"
    TRANSMUTATION = "transmutation"
    ILLUSION = "illusion"
    INVOCATION = "invocation"
    NECROMANCY = "necromancy"


class SpellLevelEnum(IntEnum, Enum):
    """Spell level options."""

    CANTRIP = 0
    FIRST = 1
    SECOND = 2
    THIRD = 3
    FOURTH = 4
    FIFTH = 5
    SIXTH = 6
    SEVENTH = 7
    EIGHTH = 8
    NINTH = 9


class DamageTypeEnum(StrEnum):
    """Damage type options."""

    # special cases
    MIXED = "mixed"  # e.g. absorb elements can absorb and return any type
    SPECIAL = "special"  # e.g. hunter's mark where = weapon's damage or absorb elements = reflects an element type
    # actual types
    ACID = "acid"
    BLUDGEONING = "bludgeoning"
    COLD = "cold"
    FIRE = "fire"
    FORCE = "force"
    LIGHTING = "lightning"
    NECROTIC = "necrotic"
    PIERCING = "piercing"
    POISON = "poison"
    PSYCHIC = "psychic"
    RADIANT = "radiant"
    SLASHING = "slashing"
    THUNDER = "thunder"


class AbilityScoreEnum(StrEnum):
    """Ability score options."""

    STR = "str"
    DEX = "dex"
    CON = "con"
    INT = "int"
    WIS = "wis"
    CHA = "cha"
