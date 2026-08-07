#!/usr/bin/env python3
"""Read-only validator and deterministic reference model for Iris Online drops.

The model follows the relevant control flow in the supplied Drop.cpp and
DropScript.cpp reference sources. It preserves row order, uses the server's
1..1,000,000 roll scale, models base/additional attempts, cumulative group/item
weights, neutral level penalty, time-restriction weight transforms, duplicate
row prevention inside one rule cycle, field/instance gating and fallback/event
attempt counts. It deliberately does not invent a final per-kill probability
when runtime state is required.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from collections import Counter, defaultdict
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Iterable, Mapping, MutableSet, Sequence

CHANCE_SCALE = 1_000_000


@dataclass(frozen=True)
class WeightedEntry:
    identifier: int
    weight: int

    @property
    def chance(self) -> int:  # compatibility with earlier audit callers
        return self.weight


@dataclass(frozen=True)
class DropRule:
    source_line: int
    owner_id: int
    add1_count: int
    add1_rate: int
    add2_count: int
    add2_rate: int
    entries: tuple[WeightedEntry, ...]

    @property
    def weight_total(self) -> int:
        return sum(entry.weight for entry in self.entries)

    @property
    def chance_total(self) -> int:  # compatibility
        return self.weight_total

    @property
    def empty_weight(self) -> int:
        return max(0, CHANCE_SCALE - self.weight_total)


@dataclass(frozen=True)
class WorldDropRule(DropRule):
    min_level: int
    max_level: int
    server_type_check: int
    monster_type: int


@dataclass(frozen=True)
class GroupItem:
    source_line: int
    group_id: int
    position: int
    item_id: int
    weight: int
    quantity: int

    @property
    def chance(self) -> int:  # compatibility
        return self.weight

    @property
    def row_key(self) -> tuple[int, int]:
        # Drop.cpp stores the selected sDropItemScript pointer. In a deterministic
        # table model, group+source-order identifies that script row.
        return self.group_id, self.position


@dataclass(frozen=True)
class QuestDrop:
    source_line: int
    quest_id: int
    monster_id: int
    item_id: int
    rate_percent: int


@dataclass(frozen=True)
class DropRestriction:
    source_line: int
    item_id: int
    drop_term_ms: int
    delay_rate_min: float
    delay_rate_max: float
    weight_am: float
    weight_pm: float


@dataclass(frozen=True)
class ItemPickResult:
    status: str  # selected | duplicate | none
    item_id: int | None
    quantity: int
    source_line: int | None
    cumulative_weight: int


@dataclass
class AuditResult:
    normal_monsters: int
    normal_rows: int
    normal_row_count_distribution: dict[int, int]
    normal_more_than_seven: list[dict]
    normal_overflow_rows: list[dict]
    normal_duplicate_group_rows: list[dict]
    normal_rows_with_additional_attempts: int
    world_rows: int
    world_overflow_rows: list[dict]
    world_duplicate_group_rows: list[dict]
    world_rows_with_additional_attempts: int
    world_server_type_values: dict[int, int]
    groups: int
    group_overflow: list[dict]
    group_duplicate_items: list[dict]
    quantity_over_one_rows: int
    missing_group_references: list[dict]
    restricted_items: int
    penalty_rows: int
    quest_rows: int
    quest_monsters: int
    reference_sources: dict[str, str]


def _int(text: str, default: int = 0) -> int:
    try:
        return int(text.strip())
    except (AttributeError, ValueError):
        return default


def _positive_int(text: str) -> int | None:
    value = _int(text)
    return value if value > 0 else None


def _float(text: str, default: float = 0.0) -> float:
    try:
        return float(text.strip())
    except (AttributeError, ValueError):
        return default


def parse_pairs(columns: Sequence[str], start: int) -> tuple[WeightedEntry, ...]:
    entries: list[WeightedEntry] = []
    for index in range(start, len(columns) - 1, 2):
        identifier = _positive_int(columns[index])
        weight = _positive_int(columns[index + 1])
        if identifier is not None and weight is not None:
            entries.append(WeightedEntry(identifier, weight))
    return tuple(entries)


def parse_normal(path: Path) -> dict[int, list[DropRule]]:
    rows: dict[int, list[DropRule]] = defaultdict(list)
    with path.open("r", encoding="utf-8-sig", errors="replace", newline="") as source:
        for source_line, raw_line in enumerate(source, 1):
            columns = raw_line.rstrip("\r\n").split("\t")
            if len(columns) < 8 or columns[0].strip().lower() != "monster":
                continue
            monster_id = _positive_int(columns[1])
            entries = parse_pairs(columns, 6)
            if monster_id is None or not entries:
                continue
            rows[monster_id].append(DropRule(
                source_line=source_line,
                owner_id=monster_id,
                add1_count=max(0, _int(columns[2])),
                add1_rate=max(0, _int(columns[3])),
                add2_count=max(0, _int(columns[4])),
                add2_rate=max(0, _int(columns[5])),
                entries=entries,
            ))
    return dict(rows)


def parse_world(path: Path) -> list[WorldDropRule]:
    rows: list[WorldDropRule] = []
    with path.open("r", encoding="utf-8-sig", errors="replace", newline="") as source:
        for source_line, raw_line in enumerate(source, 1):
            columns = raw_line.rstrip("\r\n").split("\t")
            if len(columns) < 11 or columns[0].strip().lower() != "world":
                continue
            entries = parse_pairs(columns, 9)
            if not entries:
                continue
            rows.append(WorldDropRule(
                source_line=source_line,
                owner_id=source_line,
                min_level=_int(columns[1]),
                max_level=_int(columns[2]),
                server_type_check=_int(columns[3]),
                monster_type=_int(columns[4]),
                add1_count=max(0, _int(columns[5])),
                add1_rate=max(0, _int(columns[6])),
                add2_count=max(0, _int(columns[7])),
                add2_rate=max(0, _int(columns[8])),
                entries=entries,
            ))
    return rows


def parse_groups(path: Path) -> dict[int, list[GroupItem]]:
    groups: dict[int, list[GroupItem]] = defaultdict(list)
    with path.open("r", encoding="utf-8-sig", errors="replace", newline="") as source:
        for source_line, raw_line in enumerate(source, 1):
            columns = raw_line.rstrip("\r\n").split("\t")
            if len(columns) < 4:
                continue
            group_id = _positive_int(columns[0])
            item_id = _positive_int(columns[1])
            weight = _positive_int(columns[2])
            quantity = _positive_int(columns[3])
            if None in (group_id, item_id, weight, quantity):
                continue
            group = groups[group_id]
            group.append(GroupItem(source_line, group_id, len(group) + 1, item_id, weight, quantity))
    return dict(groups)


def parse_quest(path: Path | None) -> list[QuestDrop]:
    if path is None:
        return []
    rows: list[QuestDrop] = []
    with path.open("r", encoding="utf-8-sig", errors="replace", newline="") as source:
        for source_line, raw_line in enumerate(source, 1):
            columns = raw_line.rstrip("\r\n").split("\t")
            if len(columns) < 4:
                continue
            quest_id = _positive_int(columns[0])
            monster_id = _positive_int(columns[1])
            item_id = _positive_int(columns[2])
            rate = _positive_int(columns[3])
            if None in (quest_id, monster_id, item_id, rate):
                continue
            rows.append(QuestDrop(source_line, quest_id, monster_id, item_id, min(100, rate)))
    return rows


def quest_roll_selects(drop_rate_percent: int, roll_1_100: int, *, conditions_met: bool = True) -> bool:
    """Probability gate for one quest-drop row after active-quest lookup."""
    if not 1 <= roll_1_100 <= 100:
        raise ValueError("roll_1_100 must be in 1..100")
    if drop_rate_percent < 0:
        raise ValueError("drop_rate_percent must be non-negative")
    return conditions_met and roll_1_100 <= min(100, drop_rate_percent)


def parse_restrictions(path: Path | None) -> dict[int, DropRestriction]:
    if path is None:
        return {}
    result: dict[int, DropRestriction] = {}
    with path.open("r", encoding="utf-8-sig", errors="replace", newline="") as source:
        for source_line, raw_line in enumerate(source, 1):
            columns = raw_line.rstrip("\r\n").split("\t")
            if len(columns) < 6:
                continue
            item_id = _positive_int(columns[0])
            if item_id is None:
                continue
            result[item_id] = DropRestriction(
                source_line=source_line,
                item_id=item_id,
                drop_term_ms=max(0, _int(columns[1])),
                delay_rate_min=max(0.0, _float(columns[2])),
                delay_rate_max=max(0.0, _float(columns[3])),
                weight_am=max(0.0, _float(columns[4], 1.0)),
                weight_pm=max(0.0, _float(columns[5], 1.0)),
            )
    return result


def parse_penalties(path: Path | None) -> dict[int, dict[int, float]]:
    """Parse DropIncrease.txt into {0: field, 1: instance/theme} tables."""
    if path is None:
        return {}
    tokens = path.read_text(encoding="utf-8-sig", errors="replace").replace("{", " { ").replace("}", " } ").split()
    result: dict[int, dict[int, float]] = defaultdict(dict)
    index = 0
    while index < len(tokens):
        try:
            kind = int(tokens[index])
        except ValueError:
            index += 1
            continue
        index += 1
        if index >= len(tokens) or tokens[index] != "{":
            continue
        index += 1
        while index < len(tokens) and tokens[index] != "}":
            if index + 1 >= len(tokens):
                break
            try:
                diff = int(tokens[index])
                value = float(tokens[index + 1])
            except ValueError:
                index += 1
                continue
            result[kind][diff] = value
            index += 2
        index += 1
    return {kind: dict(values) for kind, values in result.items()}


def weighted_pick(entries: Sequence[WeightedEntry], roll: int) -> int | None:
    """Server cumulative choice for a 1..1,000,000 roll."""
    if not 1 <= roll <= CHANCE_SCALE:
        raise ValueError(f"roll must be in 1..{CHANCE_SCALE}")
    cumulative = 0
    for entry in entries:
        cumulative += entry.weight
        if cumulative < roll:
            continue
        return entry.identifier
    return None


def effective_interval_weight(entries: Sequence[WeightedEntry], position: int) -> int:
    """Reachable neutral weight for one ordered entry on the 1..scale roll."""
    if position < 0 or position >= len(entries):
        raise IndexError(position)
    before = sum(max(0, entry.weight) for entry in entries[:position])
    after = before + max(0, entries[position].weight)
    return max(0, min(CHANCE_SCALE, after) - min(CHANCE_SCALE, before))


def additional_attempts(rule: DropRule, roll: int, *, world: bool = False) -> int:
    """Return total attempts including the server's unconditional base attempt."""
    if not 1 <= roll <= CHANCE_SCALE:
        raise ValueError(f"roll must be in 1..{CHANCE_SCALE}")
    total_rate = rule.add1_rate + rule.add2_rate
    if world:
        total_rate = min(CHANCE_SCALE, total_rate)
    if total_rate != 0:
        if roll <= rule.add1_rate:
            return 1 + rule.add1_count
        if roll <= total_rate:
            return 1 + rule.add2_count
    return 1


def event_attempts(drop_add_percent: int, roll_0_99: int) -> int:
    """Drop.cpp fallback/event attempt count from mDropAddPer."""
    if drop_add_percent < 0:
        raise ValueError("drop_add_percent must be non-negative")
    if not 0 <= roll_0_99 <= 99:
        raise ValueError("roll_0_99 must be in 0..99")
    count, remainder = divmod(drop_add_percent, 100)
    if remainder > roll_0_99:
        count += 1
    return count


def world_rule_applies(server_type_check: int, *, is_normal_map: bool) -> bool:
    """Drop.cpp field/instance gate; it does not identify a concrete dungeon."""
    # eDROPSERVERTYPE_FIELD and eDROPSERVERTYPE_INDUN are 1/2 in the supplied tables.
    if server_type_check == 1 and not is_normal_map:
        return False
    if server_type_check == 2 and is_normal_map:
        return False
    return True


def reference_item_pick(
    items: Sequence[GroupItem],
    roll: int,
    *,
    penalty: float = 1.0,
    restrictions: Mapping[int, DropRestriction] | None = None,
    disabled_restricted_items: set[int] | None = None,
    period: str = "am",
    selected_rows: MutableSet[tuple[int, int]] | None = None,
) -> ItemPickResult:
    """Reproduce the essential ordered item selection in DropMonItemSelect.

    Runtime timestamps are represented by disabled_restricted_items. When a
    restricted row is eligible, its AM/PM multiplier is applied to the whole
    cumulative boundary exactly as CheckDropMonCantTime mutates rOutRate.
    """
    if not 1 <= roll <= CHANCE_SCALE:
        raise ValueError(f"roll must be in 1..{CHANCE_SCALE}")
    if penalty < 0:
        raise ValueError("penalty must be non-negative")
    restrictions = restrictions or {}
    disabled_restricted_items = disabled_restricted_items or set()
    selected_rows = selected_rows if selected_rows is not None else set()
    if period not in {"am", "pm"}:
        raise ValueError("period must be 'am' or 'pm'")

    cumulative = 0
    for item in items:
        cumulative += int(item.weight * penalty)
        restriction = restrictions.get(item.item_id)
        if restriction is not None:
            if item.item_id in disabled_restricted_items:
                # Server continues after the failed time check, preserving the
                # cumulative boundary accumulated so far.
                continue
            multiplier = restriction.weight_am if period == "am" else restriction.weight_pm
            cumulative = int(cumulative * multiplier)
        if cumulative < roll:
            continue
        if item.row_key in selected_rows:
            # DropMonItemSelect returns false immediately; it does not continue
            # to a later row after selecting a duplicate script row.
            return ItemPickResult("duplicate", item.item_id, item.quantity, item.source_line, cumulative)
        selected_rows.add(item.row_key)
        return ItemPickResult("selected", item.item_id, item.quantity, item.source_line, cumulative)
    return ItemPickResult("none", None, 0, None, cumulative)


def _row_dict(row: DropRule) -> dict:
    return {
        "sourceLine": row.source_line,
        "ownerId": row.owner_id,
        "baseAttempts": 1,
        "additionalAttempts1": {"count": row.add1_count, "rate": row.add1_rate},
        "additionalAttempts2": {"count": row.add2_count, "rate": row.add2_rate},
        "weightTotal": row.weight_total,
        "entries": [asdict(entry) for entry in row.entries],
    }


def _duplicates(values: Iterable[int]) -> bool:
    values = list(values)
    return len(set(values)) != len(values)


def _sha256(path: Path | None) -> str:
    if path is None or not path.is_file():
        return ""
    return hashlib.sha256(path.read_bytes()).hexdigest()


def audit(
    normal_path: Path,
    groups_path: Path,
    world_path: Path,
    restrictions_path: Path | None = None,
    penalty_path: Path | None = None,
    quest_path: Path | None = None,
    reference_drop_cpp: Path | None = None,
    reference_drop_script: Path | None = None,
) -> AuditResult:
    normal = parse_normal(normal_path)
    world = parse_world(world_path)
    groups = parse_groups(groups_path)
    restrictions = parse_restrictions(restrictions_path)
    penalties = parse_penalties(penalty_path)
    quests = parse_quest(quest_path)

    normal_rows = [row for rows in normal.values() for row in rows]
    normal_overflow = [_row_dict(row) for row in normal_rows if row.weight_total > CHANCE_SCALE]
    normal_duplicate = [_row_dict(row) for row in normal_rows if _duplicates(entry.identifier for entry in row.entries)]
    world_overflow = [_row_dict(row) for row in world if row.weight_total > CHANCE_SCALE]
    world_duplicate = [_row_dict(row) for row in world if _duplicates(entry.identifier for entry in row.entries)]

    group_overflow: list[dict] = []
    group_duplicate_items: list[dict] = []
    quantity_over_one = 0
    for group_id, items in groups.items():
        weight_total = sum(item.weight for item in items)
        record = {"groupId": group_id, "weightTotal": weight_total, "entries": len(items)}
        if weight_total > CHANCE_SCALE:
            group_overflow.append(record)
        if _duplicates(item.item_id for item in items):
            group_duplicate_items.append(record)
        quantity_over_one += sum(1 for item in items if item.quantity > 1)

    missing: list[dict] = []
    for source_kind, rows in (("normal", normal_rows), ("world", world)):
        for row in rows:
            for entry in row.entries:
                if entry.identifier not in groups:
                    missing.append({
                        "source": source_kind,
                        "sourceLine": row.source_line,
                        "ownerId": row.owner_id,
                        "groupId": entry.identifier,
                    })

    distribution = Counter(len(rows) for rows in normal.values())
    more_than_seven = [
        {"monsterId": monster_id, "rows": len(rows), "sourceLines": [row.source_line for row in rows]}
        for monster_id, rows in sorted(normal.items()) if len(rows) > 7
    ]
    server_type_values = Counter(row.server_type_check for row in world)
    return AuditResult(
        normal_monsters=len(normal),
        normal_rows=len(normal_rows),
        normal_row_count_distribution=dict(sorted(distribution.items())),
        normal_more_than_seven=more_than_seven,
        normal_overflow_rows=normal_overflow,
        normal_duplicate_group_rows=normal_duplicate,
        normal_rows_with_additional_attempts=sum(1 for row in normal_rows if row.add1_count or row.add1_rate or row.add2_count or row.add2_rate),
        world_rows=len(world),
        world_overflow_rows=world_overflow,
        world_duplicate_group_rows=world_duplicate,
        world_rows_with_additional_attempts=sum(1 for row in world if row.add1_count or row.add1_rate or row.add2_count or row.add2_rate),
        world_server_type_values=dict(sorted(server_type_values.items())),
        groups=len(groups),
        group_overflow=group_overflow,
        group_duplicate_items=group_duplicate_items,
        quantity_over_one_rows=quantity_over_one,
        missing_group_references=missing,
        restricted_items=len(restrictions),
        penalty_rows=sum(len(table) for table in penalties.values()),
        quest_rows=len(quests),
        quest_monsters=len({row.monster_id for row in quests}),
        reference_sources={
            "Drop.cpp": _sha256(reference_drop_cpp),
            "DropScript.cpp": _sha256(reference_drop_script),
            "Item_DropN.txt": _sha256(normal_path),
            "Item_DropW.txt": _sha256(world_path),
            "item_droplist.txt": _sha256(groups_path),
            "item_droplimit.txt": _sha256(restrictions_path),
            "DropIncrease.txt": _sha256(penalty_path),
            "Item_DropQ.txt": _sha256(quest_path),
        },
    )


def markdown_report(result: AuditResult) -> str:
    lines = [
        "# Проверка таблиц выпадения",
        "",
        f"- Монстров с monster-specific дропом: {result.normal_monsters}",
        f"- Строк Item_DropN: {result.normal_rows}",
        f"- Строк Item_DropN с дополнительными попытками: {result.normal_rows_with_additional_attempts}",
        f"- Строк Item_DropW: {result.world_rows}",
        f"- Строк Item_DropW с дополнительными попытками: {result.world_rows_with_additional_attempts}",
        f"- Групп item_droplist: {result.groups}",
        f"- Строк quantity > 1: {result.quantity_over_one_rows}",
        f"- Ограниченных item ID: {result.restricted_items}",
        f"- Строк drop penalty: {result.penalty_rows}",
        f"- Строк Item_DropQ: {result.quest_rows} (монстров: {result.quest_monsters})",
        f"- Item_DropN overflow > 1 000 000: {len(result.normal_overflow_rows)}",
        f"- Item_DropW overflow > 1 000 000: {len(result.world_overflow_rows)}",
        f"- item_droplist overflow > 1 000 000: {len(result.group_overflow)}",
        f"- Ссылок на отсутствующие группы: {len(result.missing_group_references)}",
        "",
        "Reference model использует шкалу 1..1 000 000 и накопительные границы сервера.",
        "Field/instance рассматривается только как условие ветки Item_DropW и не используется как связь с конкретным подземельем.",
        "Точный шанс за убийство не выводится из этих таблиц без runtime penalty, состояния временных ограничений, AM/PM и drop-add/event modifiers.",
    ]
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--normal", type=Path, required=True)
    parser.add_argument("--groups", type=Path, required=True)
    parser.add_argument("--world", type=Path, required=True)
    parser.add_argument("--limits", type=Path)
    parser.add_argument("--penalty", type=Path)
    parser.add_argument("--quest", type=Path)
    parser.add_argument("--reference-drop-cpp", type=Path)
    parser.add_argument("--reference-drop-script", type=Path)
    parser.add_argument("--json", type=Path)
    parser.add_argument("--report", type=Path)
    parser.add_argument("--strict", action="store_true", help="return non-zero when structural anomalies are found")
    args = parser.parse_args()

    result = audit(
        args.normal, args.groups, args.world, args.limits, args.penalty, args.quest,
        args.reference_drop_cpp, args.reference_drop_script,
    )
    payload = asdict(result)
    if args.json:
        args.json.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    else:
        print(json.dumps(payload, ensure_ascii=False, indent=2))
    if args.report:
        args.report.write_text(markdown_report(result), encoding="utf-8")

    anomalies = (
        result.normal_more_than_seven
        or result.normal_overflow_rows
        or result.normal_duplicate_group_rows
        or result.world_overflow_rows
        or result.world_duplicate_group_rows
        or result.group_overflow
        or result.missing_group_references
    )
    return 2 if args.strict and anomalies else 0


if __name__ == "__main__":
    raise SystemExit(main())
