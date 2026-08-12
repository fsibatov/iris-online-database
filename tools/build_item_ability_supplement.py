"""Build additive item details omitted by the legacy game-data projection.

Existing JSON keys in game_data.json.gz are authoritative and never overwritten.
The generated supplement only restores data that is absent from that projection,
plus raw technical/restriction fields for which the embedded JSON has no key.
"""

from __future__ import annotations

import argparse
import gzip
import json
from pathlib import Path

ABILITY_FIELDS = {
    "physicalDefense": 6,
    "magicDefense": 7,
    "attackRange": 8,
    "attackSpeed": 9,
    "cooldown": 10,
    "physicalMin": 12,
    "physicalMax": 13,
    "magicMin": 14,
    "magicMax": 15,
    "heal": 16,
}
ABILITY_META_FIELDS = {
    "abilityDescriptionIndex": 1,
    "defenseType": 2,
    "rangeType": 3,
    "targetType": 4,
    "useRange": 5,
    "groupTime": 11,
    "influenceIndex": 27,
    "activeIndex": 28,
}
DEFINE_META_FIELDS = {
    "nameIndex": 2,
    "tooltipIndex": 3,
    "abilityIndex": 22,
    "cardIndex": 24,
    "kindOf": 4,
    "eventType": 5,
    "buyCurrency": 13,
    "buyPrice": 14,
    "maxInventory": 16,
    "termSet": 17,
    "termDuration": 18,
    "printableFlag": 19,
    "limitIndex": 23,
    "tarotIndex": 25,
    "spreadIndex": 26,
    "degradationIndex": 27,
    "cardSlotIndex": 28,
    "enhanceProbabilityIndex": 29,
    "enhancedIndex": 30,
    "reinforcingIndex": 31,
    "changeIndex": 32,
    "titleIndex": 33,
    "modelIndex": 36,
    "modelLeftIndex": 37,
    "gwipyosi": 38,
}
LIMIT_FIELDS = {
    "useMapType": 7,
    "makeSkill": 8,
    "makeSkillExp": 9,
    "guildUse": 10,
    "limitMapTypeRaw": 11,
    "limitValueRaw": 12,
    "limitExtraRaw": 13,
}
OPTION_POSITIONS = (17, 19, 21, 23, 25)


def safe_int(value):
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def read_rows(path: Path):
    for raw in path.read_text(encoding="cp1251", errors="replace").splitlines():
        if not raw or raw.startswith("//"):
            continue
        yield raw.split("\t")


def read_indexed_text(path: Path) -> dict[int, str]:
    result: dict[int, str] = {}
    for raw in path.read_text(encoding="utf-16", errors="strict").splitlines():
        if not raw or raw.startswith("//"):
            continue
        parts = raw.split("\t", 1)
        if len(parts) != 2:
            continue
        index = safe_int(parts[0].strip())
        if index is None:
            continue
        result[index] = parts[1].strip().strip('"')
    return result


def load_game(path: Path):
    with gzip.open(path, "rt", encoding="utf-8") as handle:
        return json.load(handle)


def build(
    game_data: Path,
    item_define: Path,
    item_ability: Path,
    item_limit: Path,
    item_tooltips: Path,
    skill_effects: Path,
):
    game = load_game(game_data)
    items = {row["id"]: row for row in game["items"]}
    tooltips = read_indexed_text(item_tooltips)

    influence_durations: dict[int, int] = {}
    for parts in read_rows(skill_effects):
        influence_id = safe_int(parts[0]) if parts else None
        duration_ms = safe_int(parts[15]) if len(parts) > 15 else None
        if influence_id is not None and duration_ms is not None:
            influence_durations[influence_id] = duration_ms

    define = {}
    for parts in read_rows(item_define):
        index = safe_int(parts[0]) if parts else None
        if index is not None:
            define[index] = parts

    abilities = {}
    for parts in read_rows(item_ability):
        index = safe_int(parts[0]) if parts else None
        if index is not None:
            abilities[index] = parts

    limits = {}
    for parts in read_rows(item_limit):
        index = safe_int(parts[0]) if parts else None
        if index is not None:
            limits[index] = parts

    output = {}
    conflicts = []
    for item_id, item in items.items():
        source_define = define.get(item_id)
        if source_define is None:
            continue
        row = {}

        for field, position in DEFINE_META_FIELDS.items():
            value = (
                safe_int(source_define[position])
                if len(source_define) > position
                else None
            )
            if value is not None and value != 0:
                row[field] = value

        ability_id = safe_int(source_define[22]) if len(source_define) > 22 else None
        source = abilities.get(ability_id)
        if source is not None:
            for field, position in ABILITY_FIELDS.items():
                value = safe_int(source[position]) if len(source) > position else None
                if value is None:
                    continue
                if field not in item:
                    if value != 0:
                        row[field] = value
                elif item[field] != value:
                    conflicts.append(
                        {
                            "itemId": item_id,
                            "field": field,
                            "embedded": item[field],
                            "source": value,
                        }
                    )

            for field, position in ABILITY_META_FIELDS.items():
                value = safe_int(source[position]) if len(source) > position else None
                if value is not None and value != 0:
                    row[field] = value
            description_index = safe_int(source[1]) if len(source) > 1 else None
            if description_index:
                description = tooltips.get(description_index, "").strip()
                if description:
                    row["abilityDescription"] = description

            influence_index = safe_int(source[27]) if len(source) > 27 else 0
            if influence_index:
                duration_ms = influence_durations.get(influence_index)

                if duration_ms is not None and duration_ms > 0:
                    row["effectDurationMs"] = duration_ms

            source_options = []
            options_valid = True
            for position in OPTION_POSITIONS:
                option_type = (
                    safe_int(source[position]) if len(source) > position else None
                )
                option_value = (
                    safe_int(source[position + 1])
                    if len(source) > position + 1
                    else None
                )
                if option_type is None or option_value is None:
                    options_valid = False
                    break
                if option_type:
                    source_options.append({"type": option_type, "value": option_value})
            if options_valid:
                if "options" not in item:
                    if source_options:
                        row["options"] = source_options
                elif item["options"] != source_options:
                    conflicts.append(
                        {
                            "itemId": item_id,
                            "field": "options",
                            "embedded": item["options"],
                            "source": source_options,
                        }
                    )

        limit_id = safe_int(source_define[23]) if len(source_define) > 23 else None
        source_limit = limits.get(limit_id)
        if source_limit is not None:
            make_skill = safe_int(source_limit[8]) if len(source_limit) > 8 else 0
            for field, position in LIMIT_FIELDS.items():
                value = (
                    safe_int(source_limit[position])
                    if len(source_limit) > position
                    else None
                )
                if value is None:
                    continue

                if (field == "makeSkillExp" and make_skill) or value != 0:
                    row[field] = value

        if row:
            output[str(item_id)] = row

    return {"schemaVersion": 3, "items": output, "conflictsPreserved": conflicts}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--game-data", type=Path, required=True)
    parser.add_argument("--item-define", type=Path, required=True)
    parser.add_argument("--item-ability", type=Path, required=True)
    parser.add_argument("--item-limit", type=Path, required=True)
    parser.add_argument("--item-tooltips", type=Path, required=True)
    parser.add_argument("--skill-effects", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = build(
        args.game_data,
        args.item_define,
        args.item_ability,
        args.item_limit,
        args.item_tooltips,
        args.skill_effects,
    )
    args.output.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(
        data, ensure_ascii=False, separators=(",", ":"), sort_keys=False
    ).encode("utf-8")
    with (
        args.output.open("wb") as output,
        gzip.GzipFile(
            filename="", mode="wb", fileobj=output, mtime=0, compresslevel=9
        ) as gz,
    ):
        gz.write(payload)
    field_counts = {}
    for patch in data["items"].values():
        for key in patch:
            field_counts[key] = field_counts.get(key, 0) + 1
    print(
        f"items={len(data['items'])} conflictsPreserved={len(data['conflictsPreserved'])}"
    )
    print(json.dumps(dict(sorted(field_counts.items())), ensure_ascii=False))


if __name__ == "__main__":
    main()
