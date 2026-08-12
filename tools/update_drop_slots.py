"""Update slot-preserving drop rules in the embedded Iris Online data file.

The source tables are not redistributed by this project. Supply them explicitly.
Only server drop rules, exact drop-list order, and source-date metadata are
changed; item and monster records remain untouched.
"""

from __future__ import annotations

import argparse
import gzip
import json
from pathlib import Path
from typing import Any

CHANCE_SCALE = 10_000.0


def parse_pairs(columns: list[str], start: int) -> list[dict[str, Any]]:
    choices: list[dict[str, Any]] = []
    for index in range(start, len(columns) - 1, 2):
        group_text = columns[index].strip()
        chance_text = columns[index + 1].strip()
        if not group_text or not chance_text:
            continue
        try:
            group_id = int(group_text)
            raw_chance = int(chance_text)
        except ValueError:
            continue
        if group_id <= 0 or raw_chance <= 0:
            continue
        choices.append({"groupId": group_id, "chance": raw_chance / CHANCE_SCALE})
    return choices


def parse_drop_lists(path: Path) -> dict[str, list[dict[str, Any]]]:
    result: dict[str, list[dict[str, Any]]] = {}
    with path.open("r", encoding="utf-8-sig", errors="replace", newline="") as source:
        for raw_line in source:
            columns = raw_line.rstrip("\r\n").split("\t")
            if len(columns) < 4:
                continue
            try:
                group_id = int(columns[0].strip())
                item_id = int(columns[1].strip())
                raw_chance = int(columns[2].strip())
                quantity = int(columns[3].strip())
            except ValueError:
                continue
            if group_id <= 0 or item_id <= 0 or raw_chance <= 0 or quantity <= 0:
                continue
            result.setdefault(str(group_id), []).append(
                {
                    "itemId": item_id,
                    "chance": raw_chance / CHANCE_SCALE,
                    "quantity": quantity,
                }
            )
    return result


def parse_direct_slots(path: Path) -> dict[str, list[dict[str, Any]]]:
    result: dict[str, list[dict[str, Any]]] = {}
    with path.open("r", encoding="utf-8-sig", errors="replace", newline="") as source:
        for source_line, raw_line in enumerate(source, 1):
            columns = raw_line.rstrip("\r\n").split("\t")
            if len(columns) < 8 or columns[0].strip() != "monster":
                continue
            monster_id = columns[1].strip()
            try:
                if int(monster_id) <= 0:
                    continue
            except ValueError:
                continue
            choices = parse_pairs(columns, 6)
            if choices:
                result.setdefault(monster_id, []).append(
                    {
                        "sourceLine": source_line,
                        "addAttempt1Count": max(0, int(columns[2].strip() or 0)),
                        "addAttempt1Rate": max(0, int(columns[3].strip() or 0))
                        / CHANCE_SCALE,
                        "addAttempt2Count": max(0, int(columns[4].strip() or 0)),
                        "addAttempt2Rate": max(0, int(columns[5].strip() or 0))
                        / CHANCE_SCALE,
                        "choices": choices,
                    }
                )
    return result


def parse_world_rules(path: Path) -> list[dict[str, Any]]:
    rules: list[dict[str, Any]] = []
    with path.open("r", encoding="utf-8-sig", errors="replace", newline="") as source:
        for source_line, raw_line in enumerate(source, 1):
            columns = raw_line.rstrip("\r\n").split("\t")
            if len(columns) < 11 or columns[0].strip() != "world":
                continue
            try:
                min_level = int(columns[1])
                max_level = int(columns[2])
                context_id = int(columns[3])
                monster_type = int(columns[4])
            except ValueError:
                continue
            choices = parse_pairs(columns, 9)
            if not choices:
                continue
            rules.append(
                {
                    "sourceLine": source_line,
                    "minLevel": min_level,
                    "maxLevel": max_level,
                    "contextId": context_id,
                    "monsterType": monster_type,
                    "addAttempt1Count": max(0, int(columns[5].strip() or 0)),
                    "addAttempt1Rate": max(0, int(columns[6].strip() or 0))
                    / CHANCE_SCALE,
                    "addAttempt2Count": max(0, int(columns[7].strip() or 0)),
                    "addAttempt2Rate": max(0, int(columns[8].strip() or 0))
                    / CHANCE_SCALE,
                    "groups": choices,
                }
            )
    return rules


def update_server(
    server: dict[str, Any],
    direct_path: Path,
    list_path: Path,
    world_path: Path,
    direct_date: str,
    list_date: str,
    world_date: str,
) -> None:
    direct_slots = parse_direct_slots(direct_path)
    drop_lists = parse_drop_lists(list_path)
    world_rules = parse_world_rules(world_path)
    server["directSlots"] = direct_slots
    server["dropLists"] = drop_lists
    server["worldRules"] = world_rules
    server["directDropSlots"] = sum(len(slots) for slots in direct_slots.values())
    server["directDropEntries"] = sum(
        len(slot["choices"]) for slots in direct_slots.values() for slot in slots
    )
    server["dropListGroups"] = len(drop_lists)
    server["directDropsUpdatedAt"] = direct_date
    server["dropListsUpdatedAt"] = list_date
    server["worldDropsUpdatedAt"] = world_date

    server.pop("directRules", None)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--data", type=Path, required=True)
    parser.add_argument("--kiss-dropn", type=Path, required=True)
    parser.add_argument("--kiss-droplist", type=Path, required=True)
    parser.add_argument("--kiss-dropw", type=Path, required=True)
    parser.add_argument("--original-dropn", type=Path, required=True)
    parser.add_argument("--original-droplist", type=Path, required=True)
    parser.add_argument("--original-dropw", type=Path, required=True)
    parser.add_argument("--data-date", required=True, help="YYYY-MM-DD")
    parser.add_argument("--drop-date", required=True, help="YYYY-MM-DD")
    parser.add_argument("--kiss-direct-date", required=True, help="YYYY-MM-DD")
    parser.add_argument("--kiss-list-date", required=True, help="YYYY-MM-DD")
    parser.add_argument("--kiss-world-date", required=True, help="YYYY-MM-DD")
    parser.add_argument("--original-direct-date", required=True, help="YYYY-MM-DD")
    parser.add_argument("--original-list-date", required=True, help="YYYY-MM-DD")
    parser.add_argument("--original-world-date", required=True, help="YYYY-MM-DD")
    args = parser.parse_args()

    with gzip.open(args.data, "rt", encoding="utf-8") as source:
        data = json.load(source)

    before_items = json.dumps(
        data["items"], ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )
    before_monsters = json.dumps(
        data["monsters"], ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )

    update_server(
        data["servers"]["kiss"],
        args.kiss_dropn,
        args.kiss_droplist,
        args.kiss_dropw,
        args.kiss_direct_date,
        args.kiss_list_date,
        args.kiss_world_date,
    )
    update_server(
        data["servers"]["original"],
        args.original_dropn,
        args.original_droplist,
        args.original_dropw,
        args.original_direct_date,
        args.original_list_date,
        args.original_world_date,
    )
    data["meta"]["dataUpdatedAt"] = args.data_date
    data["meta"]["dropUpdatedAt"] = args.drop_date
    data["meta"]["dropNote"] = (
        "Каждая строка Item_DropN хранится как отдельное серверное правило с одной базовой попыткой "
        "и исходными полями дополнительных попыток. Группы внутри попытки выбираются накопительным весом, "
        "после чего накопительным весом выбирается предмет. Item_DropW хранит field/instance только как условие "
        "ветки, а не как связь с конкретной картой. Точный шанс за убийство зависит от runtime penalty, "
        "временных ограничений и drop-add/event состояния и не выводится из статических таблиц как простое произведение."
    )

    after_items = json.dumps(
        data["items"], ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )
    after_monsters = json.dumps(
        data["monsters"], ensure_ascii=False, sort_keys=True, separators=(",", ":")
    )
    if before_items != after_items or before_monsters != after_monsters:
        raise RuntimeError("item or monster records changed unexpectedly")

    temporary = args.data.with_suffix(args.data.suffix + ".tmp")
    with gzip.open(
        temporary, "wt", encoding="utf-8", compresslevel=9, newline="\n"
    ) as target:
        json.dump(data, target, ensure_ascii=False, separators=(",", ":"))
    temporary.replace(args.data)


if __name__ == "__main__":
    main()
