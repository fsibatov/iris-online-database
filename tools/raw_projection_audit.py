"""Compare original resource tables with the embedded published projection.

The original resource files are not distributed with the app. This tool accepts
an explicit resource directory and verifies fields that can be mapped without
inventing server/client semantics.
"""

from __future__ import annotations

import argparse
import collections
import gzip
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def as_int(value):
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def as_float(value):
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def rows(path: Path, encoding="cp1251"):
    for raw in path.read_text(encoding=encoding, errors="replace").splitlines():
        if not raw or raw.startswith("//"):
            continue
        yield raw.split("\t")


def indexed_text(path: Path | None) -> dict[int, str]:
    if path is None:
        return {}
    result = {}
    for raw in path.read_text(encoding="utf-16", errors="strict").splitlines():
        parts = raw.split("\t", 1)
        if len(parts) != 2:
            continue
        try:
            index = int(parts[0].strip())
        except ValueError:
            continue
        result[index] = parts[1].strip().strip('"')
    return result


def normalize_text(value: str) -> str:
    return " ".join(str(value or "").replace("\\n", "\n").replace("\r", "").split())


def audit(
    resource: Path,
    game_data: Path,
    item_abilities: Path | None = None,
    item_tooltips: Path | None = None,
    item_names: Path | None = None,
    monster_names: Path | None = None,
    monster_notes: Path | None = None,
    monster_details: Path | None = None,
    item_recipes: Path | None = None,
):
    with gzip.open(game_data, "rt", encoding="utf-8") as handle:
        game = json.load(handle)
    items = {row["id"]: dict(row) for row in game["items"]}
    monsters = {row["id"]: row for row in game["monsters"]}
    preserved_conflicts = []
    tooltip_text = indexed_text(item_tooltips)
    item_name_text = indexed_text(item_names)
    monster_name_text = indexed_text(monster_names)
    monster_note_text = indexed_text(monster_notes)
    if item_abilities is not None:
        with gzip.open(item_abilities, "rt", encoding="utf-8") as handle:
            supplement = json.load(handle)
        preserved_conflicts = supplement.get("conflictsPreserved", [])
        for key, patch in supplement.get("items", {}).items():
            item = items.get(int(key))
            if item is not None:
                item.update(patch)

    if monster_details is not None:
        with gzip.open(monster_details, "rt", encoding="utf-8") as handle:
            monster_supplement = json.load(handle)
        for key, patch in monster_supplement.get("monsters", {}).items():
            monster = monsters.get(int(key))
            if monster is not None:
                monster.update(patch)

    define = {}
    for parts in rows(resource / "item_define.txt"):
        index = as_int(parts[0]) if parts else None
        if index is not None:
            define[index] = parts

    card_slots = {}
    slot_names = {1: "A", 2: "B", 3: "AB", 4: "O", 5: "M", 6: "MI++++"}
    for parts in rows(resource / "item_card_slot.txt"):
        index = as_int(parts[0]) if parts else None
        if index is None or len(parts) < 6:
            continue
        slot_values = [as_int(value) for value in parts[1:6]]
        card_slots[index] = [
            slot_names.get(value, f"UNKNOWN:{value}") for value in slot_values if value
        ]

    direct_map = {
        "mainCategoryId": 6,
        "middleCategoryId": 7,
        "subCategoryId": 8,
        "weight": 9,
        "capacity": 10,
        "sellType": 11,
        "price": 12,
        "maxStack": 15,
        "exchange": 20,
        "seal": 21,
        "setIndex": 34,
        "iconIndex": 35,
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
        "qualityId": 39,
    }
    direct_mismatch = collections.Counter()
    excluded_raw = collections.Counter()
    item_name_mismatch = 0
    item_name_localization_missing = 0
    item_tooltip_mismatch = 0
    raw_item_w_nonzero = 0
    shared_items = 0
    for index, parts in define.items():
        item = items.get(index)
        if item is None:
            category = as_int(parts[6]) if len(parts) > 6 else None
            excluded_raw[category] += 1
            continue
        shared_items += 1
        name_index = as_int(parts[2]) if len(parts) > 2 else 0
        tooltip_index = as_int(parts[3]) if len(parts) > 3 else 0
        if item_name_text:
            localized_name = item_name_text.get(name_index, "").strip()
            if not localized_name:
                item_name_localization_missing += 1

                localized_name = parts[1].strip().strip('"') if len(parts) > 1 else ""
            if normalize_text(item.get("name", "")) != normalize_text(localized_name):
                item_name_mismatch += 1
        if tooltip_text and normalize_text(item.get("tooltip", "")) != normalize_text(
            tooltip_text.get(tooltip_index, "") if tooltip_index else ""
        ):
            item_tooltip_mismatch += 1
        if len(parts) > 40 and as_int(parts[40]) not in (None, 0):
            raw_item_w_nonzero += 1
        for field, position in direct_map.items():
            source = as_int(parts[position]) if len(parts) > position else None
            if source is not None and item.get(field, 0) != source:
                direct_mismatch[field] += 1
        slot_index = as_int(parts[28]) if len(parts) > 28 else None
        expected_slots = card_slots.get(slot_index, []) if slot_index else []
        if item.get("cardSlots", []) != expected_slots:
            direct_mismatch["cardSlots"] += 1

    abilities = {}
    for parts in rows(resource / "item_ability.txt"):
        index = as_int(parts[0]) if parts else None
        if index is not None:
            abilities[index] = parts
    ability_map = {
        "abilityDescriptionIndex": 1,
        "defenseType": 2,
        "rangeType": 3,
        "targetType": 4,
        "useRange": 5,
        "physicalDefense": 6,
        "magicDefense": 7,
        "attackRange": 8,
        "attackSpeed": 9,
        "cooldown": 10,
        "groupTime": 11,
        "physicalMin": 12,
        "physicalMax": 13,
        "magicMin": 14,
        "magicMax": 15,
        "heal": 16,
        "influenceIndex": 27,
        "activeIndex": 28,
    }
    ability_mismatch = collections.Counter()
    option_mismatch = 0
    ability_description_mismatch = 0
    compared_abilities = 0
    for index, item in items.items():
        parts = define.get(index)
        ability_id = as_int(parts[22]) if parts and len(parts) > 22 else None
        source = abilities.get(ability_id)
        if not source:
            continue
        compared_abilities += 1
        for field, position in ability_map.items():
            value = as_int(source[position]) if len(source) > position else None
            if value is not None and item.get(field, 0) != value:
                ability_mismatch[field] += 1
        description_index = as_int(source[1]) if len(source) > 1 else 0
        expected_description = (
            tooltip_text.get(description_index, "").strip() if description_index else ""
        )
        if tooltip_text and item.get("abilityDescription", "") != expected_description:
            ability_description_mismatch += 1
        expected_options = []
        for position in (17, 19, 21, 23, 25):
            option_type = as_int(source[position]) if len(source) > position else None
            option_value = (
                as_int(source[position + 1]) if len(source) > position + 1 else None
            )
            if option_type:
                expected_options.append(
                    {"type": option_type, "value": option_value or 0}
                )
        if item.get("options", []) != expected_options:
            option_mismatch += 1

    limits = {}
    for parts in rows(resource / "item_limit.txt"):
        index = as_int(parts[0]) if parts else None
        if index is not None:
            limits[index] = parts
    limit_map = {
        "race": 1,
        "gender": 2,
        "job1": 3,
        "job2": 4,
        "minLevel": 5,
        "maxLevel": 6,
        "useMapType": 7,
        "makeSkill": 8,
        "makeSkillExp": 9,
        "guildUse": 10,
        "limitMapTypeRaw": 11,
        "limitValueRaw": 12,
        "limitExtraRaw": 13,
    }
    limit_mismatch = collections.Counter()
    compared_limits = 0
    for index, item in items.items():
        parts = define.get(index)
        limit_id = as_int(parts[23]) if parts and len(parts) > 23 else None
        source = limits.get(limit_id)
        if not source:
            continue
        compared_limits += 1
        for field, position in limit_map.items():
            value = as_int(source[position]) if len(source) > position else None
            if value is not None and item.get(field, 0) != value:
                limit_mismatch[field] += 1

    monster_map = {
        "jobId": (2, as_int),
        "kind": (5, as_int),
        "type": (6, as_int),
        "level": (8, as_int),
        "hp": (9, as_int),
        "mp": (10, as_int),
        "exp": (11, as_int),
        "moneyBonus": (12, as_int),
        "defense": (13, as_int),
        "magicDefense": (14, as_int),
        "hit": (15, as_int),
        "evasion": (16, as_int),
        "criticalDefense": (17, as_int),
        "viewRange": (18, as_int),
        "importance": (19, as_int),
        "scale": (20, as_float),
        "attackRadius": (22, as_float),
        "walkSpeed": (28, as_int),
        "runSpeed": (29, as_int),
        "aggressive": (30, lambda v: bool(as_int(v))),
        "followRange": (33, as_int),
        "recovery": (36, as_int),
        "nameIndex": (1, as_int),
        "noteIndex": (3, as_int),
        "nameHeight": (4, as_int),
        "sourceFlag": (7, as_int),
        "effectScale": (21, as_float),
        "freeMoveRange": (23, as_int),
        "actionStopRatio": (24, as_int),
        "actionWalkRatio": (25, as_int),
        "actionRunRatio": (26, as_int),
        "actionStopTime": (27, as_int),
        "changeMonsterCheck": (31, as_int),
        "followTime": (32, as_int),
        "escapeType": (34, as_int),
        "escapePercent": (35, as_int),
        "recoveryTime": (37, as_int),
    }
    monster_mismatch = collections.Counter()
    monster_name_mismatch = 0
    monster_name_localization_missing = 0
    monster_note_mismatch = 0
    shared_monsters = 0
    raw_monster_ids = set()
    for parts in rows(resource / "monsterlist.txt"):
        index = as_int(parts[0]) if parts else None
        if index is None:
            continue
        raw_monster_ids.add(index)
        monster = monsters.get(index)
        if monster is None:
            continue
        shared_monsters += 1
        name_index = as_int(parts[1]) if len(parts) > 1 else 0
        note_index = as_int(parts[3]) if len(parts) > 3 else 0
        if monster_name_text:
            localized_name = monster_name_text.get(name_index, "").strip()
            if not localized_name:
                if name_index:
                    monster_name_localization_missing += 1
                    localized_name = "Неизвестный монстр"
                else:
                    localized_name = ""
            if normalize_text(monster.get("name", "")) != normalize_text(
                localized_name
            ):
                monster_name_mismatch += 1
        if monster_note_text and normalize_text(
            monster.get("note", "")
        ) != normalize_text(
            monster_note_text.get(note_index, "") if note_index else ""
        ):
            monster_note_mismatch += 1
        for field, (position, parser) in monster_map.items():
            source = parser(parts[position]) if len(parts) > position else None
            if source is not None and monster[field] != source:
                monster_mismatch[field] += 1

    recipe_mismatch = 0
    recipe_count = 0
    if item_recipes is not None and (resource / "item_mixed.txt").exists():
        with gzip.open(item_recipes, "rt", encoding="utf-8") as handle:
            recipe_asset = json.load(handle)
        source_recipes = {}
        for parts in rows(resource / "item_mixed.txt"):
            if not parts or parts[0].strip().lower() != "mix" or len(parts) <= 24:
                continue
            recipe_id = as_int(parts[7]) if len(parts) > 7 else None
            if not recipe_id:
                continue
            materials = []
            for position in range(24, 42, 3):
                if position >= len(parts):
                    break
                material_id = as_int(parts[position]) if parts[position].strip() else 0
                quantity = (
                    as_int(parts[position + 1])
                    if position + 1 < len(parts) and parts[position + 1].strip()
                    else 0
                )
                if material_id:
                    materials.append(
                        {"itemId": material_id, "quantity": max(1, quantity or 1)}
                    )
            source_recipes[str(recipe_id)] = materials
        recipe_count = len(source_recipes)
        asset_recipes = recipe_asset.get("recipes", {})
        recipe_mismatch = sum(
            1
            for key in set(source_recipes) | set(asset_recipes)
            if source_recipes.get(key) != asset_recipes.get(key)
        )

    return {
        "sharedItems": shared_items,
        "rawItemRowsExcludedFromEmbedded": sum(excluded_raw.values()),
        "itemNameMismatches": item_name_mismatch,
        "itemNameLocalizationMissing": item_name_localization_missing,
        "itemTooltipMismatches": item_tooltip_mismatch,
        "rawItemWNonzeroRows": raw_item_w_nonzero,
        "excludedRawItemMainCategories": dict(
            sorted((str(key), value) for key, value in excluded_raw.items())
        ),
        "directItemMismatches": dict(direct_mismatch),
        "comparedItemAbilities": compared_abilities,
        "itemAbilityMismatches": dict(ability_mismatch),
        "itemAbilityDescriptionMismatches": ability_description_mismatch,
        "itemOptionMismatchesAgainstRaw": option_mismatch,
        "preservedExplicitOptionConflicts": len(preserved_conflicts),
        "comparedItemLimits": compared_limits,
        "itemLimitMismatches": dict(limit_mismatch),
        "sharedMonsters": shared_monsters,
        "monsterNameMismatches": monster_name_mismatch,
        "monsterNameLocalizationMissing": monster_name_localization_missing,
        "monsterNoteMismatches": monster_note_mismatch,
        "rawMonstersMissingFromEmbedded": len(raw_monster_ids - set(monsters)),
        "embeddedMonstersMissingFromRaw": len(set(monsters) - raw_monster_ids),
        "monsterMismatches": dict(monster_mismatch),
        "recipeRows": recipe_count,
        "recipeMismatches": recipe_mismatch,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--resource", type=Path, required=True)
    parser.add_argument(
        "--game-data", type=Path, default=ROOT / "assets/game_data.json.gz"
    )
    parser.add_argument(
        "--item-abilities", type=Path, default=ROOT / "assets/item_abilities.json.gz"
    )
    parser.add_argument("--item-tooltips", type=Path)
    parser.add_argument("--item-names", type=Path)
    parser.add_argument("--monster-names", type=Path)
    parser.add_argument("--monster-notes", type=Path)
    parser.add_argument(
        "--monster-details", type=Path, default=ROOT / "assets/monster_details.json.gz"
    )
    parser.add_argument(
        "--item-recipes", type=Path, default=ROOT / "assets/item_recipes.json.gz"
    )
    args = parser.parse_args()
    result = audit(
        args.resource,
        args.game_data,
        args.item_abilities,
        args.item_tooltips,
        args.item_names,
        args.monster_names,
        args.monster_notes,
        args.monster_details,
        args.item_recipes,
    )
    print(json.dumps(result, ensure_ascii=False, indent=2))
    fatal = any(
        result[key]
        for key in (
            "directItemMismatches",
            "itemAbilityMismatches",
            "itemLimitMismatches",
            "monsterMismatches",
        )
    )
    if any(
        result[key]
        for key in (
            "itemNameMismatches",
            "itemTooltipMismatches",
            "monsterNameMismatches",
            "monsterNoteMismatches",
            "rawItemWNonzeroRows",
            "recipeMismatches",
        )
    ):
        fatal = True
    if result["itemAbilityDescriptionMismatches"]:
        fatal = True
    if (
        result["itemOptionMismatchesAgainstRaw"]
        != result["preservedExplicitOptionConflicts"]
    ):
        fatal = True
    if fatal:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
