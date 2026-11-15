"""API enum values."""

from enum import StrEnum


class DbSchemaEnum(StrEnum):
    """Database Schema Options."""

    DND = "dnd"


class SpellSchoolEnum(StrEnum):
    """Spell School Options."""

    ABJURATION = "abjuration"
    CONJURATION = "conjuration"
    DIVININATION = "divination"
    ENCHANTMENT = "enchantment"
    ILLUSION = "illusion"
    EVOCATION = "evocation"  # originally 'invocation'
    NECROMANCY = "necromancy"
    TRANSMUTATION = "transmutation"  # originally 'alteration'


class SpellLevelEnum(StrEnum):
    """Spell Level Options."""

    # Strings on purpose: the old `(str, Enum)` silently turned 0 into "0"; StrEnum requires str
    CANTRIP = "0"
    FIRST_LEVEL = "1"
    SECOND_LEVEL = "2"
    THIRD_LEVEL = "3"
    FOURTH_LEVEL = "4"
    FITH_LEVEL = "5"
    SIXTH_LEVEL = "6"
    SEVENTH_LEVEL = "7"
    EIGTH_LEVEL = "8"
    NINETH_LEVEL = "9"
