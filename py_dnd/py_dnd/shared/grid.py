"""Table filters and sorts in AG Grid's shapes: ``?filter=`` (filter model) and ``?sort=`` (sort model).

Both are JSON in a query param, so list reads stay GETs::

    ?filter={"level":{"filterType":"number","type":"lessThanOrEqual","filter":3}}
    ?sort=[{"colId":"level","sort":"asc"}]

Each route lists the columns a client may filter and sort (``GridColumn``), and each column's kind
fixes the filter it takes. Anything else is a 422. One condition per column for now: AG Grid's
``operator`` + ``conditions`` (two conditions joined by AND/OR) isn't accepted yet.

Kinds and their AG Grid filters (``app*`` ones are ours, AG Grid Community has no equivalent):

* text: ``agTextColumnFilter``, case-insensitive like AG Grid's
* number: ``agNumberColumnFilter``; ``inRange`` includes both ends (AG Grid's ``inRangeInclusive``)
* date: ``agDateColumnFilter`` on a date or timestamp column, compared by day
* year: ``agDateColumnFilter`` with year granularity, on an integer year column
* boolean: ``appBooleanColumnFilter``, ``{"filterType":"boolean","type":"true"}``
* set: ``appSetColumnFilter`` / ``appApiSetColumnFilter``, AG Grid's set model ``{"values":[...]}``
"""

import datetime
import json
import operator
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass
from enum import StrEnum
from typing import Annotated, Any, Literal

from fastapi import HTTPException, Query, status
from pydantic import BaseModel, ConfigDict, Field, TypeAdapter, ValidationError
from sqlalchemy import ColumnElement, Date, Numeric, cast, false, func, literal, or_
from sqlalchemy.sql.elements import UnaryExpression

FILTER_DESCRIPTION = (
    'AG Grid filter model as JSON, e.g. {"level":{"filterType":"number","type":"lessThanOrEqual","filter":3}}'
)
SORT_DESCRIPTION = 'AG Grid sort model as JSON, e.g. [{"colId":"level","sort":"asc"}]'


class ColumnKind(StrEnum):
    """What a column holds, which fixes the filter it takes."""

    TEXT = "text"
    NUMBER = "number"
    DATE = "date"
    YEAR = "year"
    BOOLEAN = "boolean"
    SET = "set"


class _Model(BaseModel):
    model_config = ConfigDict(extra="forbid")


class TextFilterModel(_Model):
    """agTextColumnFilter."""

    filterType: Literal["text"]
    type: Literal["contains", "notContains", "equals", "notEqual", "startsWith", "endsWith", "blank", "notBlank"]
    filter: str | None = None


class NumberFilterModel(_Model):
    """agNumberColumnFilter."""

    filterType: Literal["number"]
    type: Literal[
        "equals",
        "notEqual",
        "lessThan",
        "lessThanOrEqual",
        "greaterThan",
        "greaterThanOrEqual",
        "inRange",
        "blank",
        "notBlank",
    ]
    filter: float | None = None
    filterTo: float | None = None


class DateFilterModel(_Model):
    """agDateColumnFilter. Dates are ``YYYY-MM-DD`` (AG Grid adds `` hh:mm:ss``, which is ignored)."""

    filterType: Literal["date"]
    type: Literal["equals", "notEqual", "lessThan", "greaterThan", "inRange", "blank", "notBlank"]
    dateFrom: str | None = None
    dateTo: str | None = None


class BooleanFilterModel(_Model):
    """appBooleanColumnFilter."""

    filterType: Literal["boolean"]
    type: Literal["true", "false", "blank", "notBlank"]


class SetFilterModel(_Model):
    """appSetColumnFilter / appApiSetColumnFilter (AG Grid's set filter model)."""

    filterType: Literal["set"]
    values: list[str | int | None]


FilterModelItem = Annotated[
    TextFilterModel | NumberFilterModel | DateFilterModel | BooleanFilterModel | SetFilterModel,
    Field(discriminator="filterType"),
]


class SortModelItem(_Model):
    """One column of AG Grid's sort model."""

    colId: str
    sort: Literal["asc", "desc"]


_FILTER_MODEL = TypeAdapter(dict[str, FilterModelItem])
_SORT_MODEL = TypeAdapter(list[SortModelItem])

# the filterType each kind takes
_FILTER_TYPE = {
    ColumnKind.TEXT: "text",
    ColumnKind.NUMBER: "number",
    ColumnKind.DATE: "date",
    ColumnKind.YEAR: "date",
    ColumnKind.BOOLEAN: "boolean",
    ColumnKind.SET: "set",
}


@dataclass(frozen=True)
class GridColumn:
    """A column clients may filter and sort.

    Args:
        expression: the SQL column (or expression) to filter and sort on
        kind: what it holds
        sortable: allow ``?sort=`` on it
        set_condition: for set columns that aren't a plain ``IN`` (e.g. a related table's slug)
    """

    expression: Any
    kind: ColumnKind
    sortable: bool = True
    set_condition: Callable[[list[Any]], ColumnElement[bool]] | None = None


@dataclass(frozen=True)
class GridQuery:
    """The parsed ``?filter=`` and ``?sort=``, ready for a select."""

    where: list[ColumnElement[bool]]
    order_by: list[UnaryExpression[Any]]


def _unprocessable(detail: str) -> HTTPException:
    return HTTPException(status.HTTP_422_UNPROCESSABLE_CONTENT, detail)


def _like_escape(value: str) -> str:
    """Make %, _ and the backslash literal in a LIKE pattern."""
    return value.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")


# LIKE patterns by text filter type ({} is the escaped value); notContains negates contains
_LIKE = {"contains": "%{}%", "notContains": "%{}%", "startsWith": "{}%", "endsWith": "%{}"}

# comparisons the number, date and year filters share (inRange is separate)
_COMPARE: dict[str, Callable[[Any, Any], ColumnElement[bool]]] = {
    "equals": operator.eq,
    "notEqual": operator.ne,
    "lessThan": operator.lt,
    "lessThanOrEqual": operator.le,
    "greaterThan": operator.gt,
    "greaterThanOrEqual": operator.ge,
}


def _text(col: Any, model: TextFilterModel) -> ColumnElement[bool]:
    if model.type in ("blank", "notBlank"):
        blank = or_(col.is_(None), col == "")
        return blank if model.type == "blank" else ~blank
    if not model.filter:
        raise _unprocessable(f"'{model.type}' needs a filter value")
    if model.type in _LIKE:
        like = col.ilike(_LIKE[model.type].format(_like_escape(model.filter)), escape="\\")
        return ~like if model.type == "notContains" else like
    equal = func.lower(col) == model.filter.lower()
    return equal if model.type == "equals" else ~equal


def _compare(col: Any, op: str, low: Any, high: Any = None) -> ColumnElement[bool]:
    """``col <op> low``, or ``low <= col <= high`` for inRange (callers put the ends in order)."""
    if op != "inRange":
        return _COMPARE[op](col, low)
    if high is None:
        raise _unprocessable("'inRange' needs both ends")
    return col.between(low, high)


def _number_value(value: float | None) -> Any:
    """Whole numbers bind as int (an integer column won't take 3.0); others as numeric."""
    if value is None or value.is_integer():
        return None if value is None else int(value)
    return literal(value, Numeric())


def _number(col: Any, model: NumberFilterModel) -> ColumnElement[bool]:
    if model.type in ("blank", "notBlank"):
        return col.is_(None) if model.type == "blank" else col.is_not(None)
    if model.filter is None:
        raise _unprocessable(f"'{model.type}' needs a filter value")
    low, high = _number_value(model.filter), _number_value(model.filterTo)
    if model.type == "inRange" and high is not None and model.filterTo is not None and model.filterTo < model.filter:
        low, high = high, low
    return _compare(col, model.type, low, high)


def _day(value: str | None) -> datetime.date | None:
    if value is None:
        return None
    try:
        return datetime.date.fromisoformat(value[:10])
    except ValueError as e:
        raise _unprocessable(f"'{value}' isn't a date (YYYY-MM-DD)") from e


def _date(col: Any, model: DateFilterModel, *, year: bool) -> ColumnElement[bool]:
    if model.type in ("blank", "notBlank"):
        return col.is_(None) if model.type == "blank" else col.is_not(None)
    low, high = _day(model.dateFrom), _day(model.dateTo)
    if low is None:
        raise _unprocessable(f"'{model.type}' needs dateFrom")
    if high is not None and high < low:
        low, high = high, low
    if year:  # an integer year column: compare the years
        return _compare(col, model.type, low.year, high.year if high else None)
    return _compare(cast(col, Date), model.type, low, high)


def _boolean(col: Any, model: BooleanFilterModel) -> ColumnElement[bool]:
    match model.type:
        case "true":
            return col.is_(True)
        case "false":
            return col.is_(False)
        case "blank":
            return col.is_(None)
        case _:
            return col.is_not(None)


def _set(column: GridColumn, model: SetFilterModel) -> ColumnElement[bool]:
    values = [v for v in model.values if v is not None]
    if column.set_condition is not None:
        condition = column.set_condition(values) if values else false()
    else:
        # the driver won't compare a text column to 3, so match the column's type
        python_type = column.expression.type.python_type
        try:
            values = [python_type(v) for v in values]
        except (TypeError, ValueError) as e:
            raise _unprocessable(f"Set values must be {python_type.__name__}s") from e
        condition = column.expression.in_(values) if values else false()
    # AG Grid lists blanks as a value (null)
    return or_(condition, column.expression.is_(None)) if None in model.values else condition


def _condition(col_id: str, column: GridColumn, model: Any) -> ColumnElement[bool]:
    expected = _FILTER_TYPE[column.kind]
    if model.filterType != expected:
        raise _unprocessable(f"Column '{col_id}' takes a {expected} filter, not {model.filterType}")
    col = column.expression
    match column.kind:
        case ColumnKind.TEXT:
            return _text(col, model)
        case ColumnKind.NUMBER:
            return _number(col, model)
        case ColumnKind.DATE | ColumnKind.YEAR:
            return _date(col, model, year=column.kind == ColumnKind.YEAR)
        case ColumnKind.BOOLEAN:
            return _boolean(col, model)
        case _:
            return _set(column, model)


def _parse(adapter: TypeAdapter[Any], raw: str, name: str) -> Any:
    try:
        return adapter.validate_python(json.loads(raw))
    except json.JSONDecodeError as e:
        raise _unprocessable(f"?{name}= isn't JSON: {e.msg}") from e
    except ValidationError as e:
        problems = "; ".join(f"{'.'.join(map(str, err['loc']))}: {err['msg']}" for err in e.errors())
        raise _unprocessable(f"?{name}= is invalid: {problems}") from e


def build_grid_query(columns: Mapping[str, GridColumn], filter_json: str | None, sort_json: str | None) -> GridQuery:
    """Turn ``?filter=`` / ``?sort=`` into SQL conditions and an order, checked against ``columns``."""
    filter_model: dict[str, Any] = _parse(_FILTER_MODEL, filter_json, "filter") if filter_json else {}
    sort_model: list[SortModelItem] = _parse(_SORT_MODEL, sort_json, "sort") if sort_json else []

    where: list[ColumnElement[bool]] = []
    for col_id, model in filter_model.items():
        column = columns.get(col_id)
        if column is None:
            raise _unprocessable(f"Can't filter on '{col_id}'. Filterable: {', '.join(sorted(columns))}")
        where.append(_condition(col_id, column, model))

    order_by: list[UnaryExpression[Any]] = []
    for item in sort_model:
        column = columns.get(item.colId)
        if column is None or not column.sortable:
            sortable = sorted(k for k, c in columns.items() if c.sortable)
            raise _unprocessable(f"Can't sort on '{item.colId}'. Sortable: {', '.join(sortable)}")
        expression = column.expression
        order_by.append((expression.asc() if item.sort == "asc" else expression.desc()).nulls_last())
    return GridQuery(where=where, order_by=order_by)


def grid_query(columns: Mapping[str, GridColumn]) -> Callable[[str | None, str | None], GridQuery]:
    """A FastAPI dependency that reads ``?filter=`` and ``?sort=`` against ``columns``."""

    def dependency(
        filter_json: str | None = Query(default=None, alias="filter", description=FILTER_DESCRIPTION),
        sort_json: str | None = Query(default=None, alias="sort", description=SORT_DESCRIPTION),
    ) -> GridQuery:
        return build_grid_query(columns, filter_json, sort_json)

    return dependency


def grid_columns(model: Any, kinds: Mapping[ColumnKind, Sequence[str]]) -> dict[str, GridColumn]:
    """Plain columns of ``model`` by kind: ``{ColumnKind.TEXT: ["name", ...], ...}``."""
    return {name: GridColumn(getattr(model, name), kind) for kind, names in kinds.items() for name in names}
