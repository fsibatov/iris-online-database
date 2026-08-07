#!/usr/bin/env python3
"""Audit Iris Online data projection and presentation coverage.

The validator starts from the packaged immutable game projection plus deterministic
supplemental assets rebuilt from the supplied source tables. It checks that every
published/supplemental field has an explicit backend and presentation destination,
and that set/recipe/monster supplemental rows are complete rather than sample-based.
"""
from __future__ import annotations

import argparse
import collections
import gzip
import hashlib
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
GAME_DATA = ROOT / "assets/game_data.json.gz"
SET_EFFECTS = ROOT / "assets/set_effects.json.gz"
ITEM_ABILITIES = ROOT / "assets/item_abilities.json.gz"
ITEM_RECIPES = ROOT / "assets/item_recipes.json.gz"
MONSTER_DETAILS = ROOT / "assets/monster_details.json.gz"
MAIN_GO = ROOT / "main.go"
APP_JS = ROOT / "web/app.js"

ITEM_PRESENTATION = {
    "id": "technical", "name": "header", "tooltip": "conditional-description",
    "mainCategoryId": "technical", "mainCategory": "technical", "middleCategoryId": "technical",
    "middleCategory": "technical", "subCategoryId": "technical", "subCategory": "technical",
    "category": "header-or-technical", "subcategory": "technical", "typeLine": "header",
    "qualityId": "technical", "quality": "rarity-badge", "weight": "base-stat",
    "capacity": "technical", "sellType": "technical-and-price", "price": "price",
    "maxStack": "base-stat", "exchange": "technical", "seal": "action-and-technical",
    "cardSlots": "card-slots", "setIndex": "set-and-technical", "iconIndex": "technical",
    "race": "technical", "raceName": "restriction-if-labelled", "gender": "technical",
    "genderName": "restriction-if-labelled", "job1": "technical", "job1Name": "class-badge",
    "job2": "technical", "job2Name": "class-badge", "minLevel": "rank-header-and-technical",
    "maxLevel": "rank-header-and-technical", "physicalDefense": "base-stat", "magicDefense": "base-stat",
    "attackRange": "base-stat", "attackSpeed": "base-stat", "cooldown": "base-stat",
    "physicalMin": "base-stat", "physicalMax": "base-stat", "magicMin": "base-stat",
    "magicMax": "base-stat", "heal": "base-stat", "options": "bonus-stats",
}

ITEM_SUPPLEMENT_PRESENTATION = {
    "physicalDefense": "base-stat", "magicDefense": "base-stat", "attackRange": "base-stat",
    "attackSpeed": "base-stat", "cooldown": "base-stat", "physicalMin": "base-stat",
    "physicalMax": "base-stat", "magicMin": "base-stat", "magicMax": "base-stat",
    "heal": "base-stat", "options": "bonus-or-neutral-unknown", "abilityDescription": "bonus-text",
    "abilityDescriptionIndex": "technical", "defenseType": "technical", "rangeType": "technical",
    "targetType": "technical", "useRange": "technical", "groupTime": "technical",
    "influenceIndex": "technical", "activeIndex": "technical", "effectDurationMs": "base-stat",
    "nameIndex": "technical", "tooltipIndex": "technical", "abilityIndex": "technical", "cardIndex": "technical",
    "useMapType": "restriction", "makeSkill": "restriction", "makeSkillExp": "restriction",
    "guildUse": "restriction", "limitMapTypeRaw": "technical", "limitValueRaw": "technical",
    "limitExtraRaw": "technical", "kindOf": "technical", "eventType": "technical",
    "buyCurrency": "technical", "buyPrice": "technical", "maxInventory": "restriction",
    "termSet": "restriction-or-technical", "termDuration": "restriction-or-technical",
    "printableFlag": "action", "limitIndex": "technical", "tarotIndex": "technical",
    "spreadIndex": "technical", "degradationIndex": "action-and-technical", "cardSlotIndex": "technical",
    "enhanceProbabilityIndex": "technical", "enhancedIndex": "action-and-technical",
    "reinforcingIndex": "technical", "changeIndex": "technical", "titleIndex": "technical",
    "modelIndex": "technical", "modelLeftIndex": "technical", "gwipyosi": "technical",
}

MONSTER_PRESENTATION = {
    "id": "technical", "name": "header", "note": "conditional-description", "jobId": "technical",
    "kind": "technical", "categoryId": "technical", "category": "header", "type": "technical",
    "typeName": "header", "level": "header", "hp": "base-stat", "mp": "base-stat",
    # Numeric EXP is intentionally technical: the server owner confirmed the numeric value is obsolete.
    "exp": "technical", "moneyBonus": "technical", "defense": "base-stat", "magicDefense": "base-stat",
    "hit": "base-stat", "evasion": "base-stat", "criticalDefense": "base-stat", "viewRange": "base-stat",
    "importance": "technical", "scale": "technical", "attackRadius": "base-stat", "walkSpeed": "base-stat",
    "runSpeed": "base-stat", "aggressive": "status-badge", "followRange": "base-stat", "recovery": "technical",
}

MONSTER_SUPPLEMENT_PRESENTATION = {
    "nameIndex": "technical", "noteIndex": "technical", "nameHeight": "technical", "sourceFlag": "technical",
    "effectScale": "technical", "freeMoveRange": "technical", "actionStopRatio": "technical",
    "actionWalkRatio": "technical", "actionRunRatio": "technical", "actionStopTime": "technical",
    "changeMonsterCheck": "technical", "followTime": "technical", "escapeType": "technical",
    "escapePercent": "technical", "recoveryTime": "technical",
}

SERVER_PRESENTATION = {
    "name": "server-selector", "questDrops": "metadata", "directRulesCount": "metadata",
    "directMonsters": "metadata", "directDropEntries": "metadata", "directDropSlots": "metadata",
    "worldRulesCount": "metadata", "dropListGroups": "metadata", "dropListEntries": "metadata",
    "changeProfiles": "metadata", "changedChangeProfiles": "metadata", "directDropsUpdatedAt": "data-date",
    "dropListsUpdatedAt": "data-date", "worldDropsUpdatedAt": "data-date", "dropLists": "drop-runtime",
    "directSlots": "drop-runtime", "worldRules": "drop-runtime",
}


def load_gzip(path: Path):
    with gzip.open(path, "rt", encoding="utf-8") as handle:
        return json.load(handle)


def json_tags(go_source: str, struct_name: str) -> set[str]:
    match = re.search(rf"type\s+{re.escape(struct_name)}\s+struct\s*\{{(.*?)\n\}}", go_source, re.S)
    if not match:
        raise AssertionError(f"Go struct {struct_name} not found")
    return set(re.findall(r'json:"([^",]+)', match.group(1)))


def all_keys(rows) -> set[str]:
    return {key for row in rows for key in row.keys()}


def audit() -> dict:
    game = load_gzip(GAME_DATA)
    sets = load_gzip(SET_EFFECTS)
    abilities = load_gzip(ITEM_ABILITIES)
    recipes = load_gzip(ITEM_RECIPES)
    monster_details = load_gzip(MONSTER_DETAILS)
    go_source = MAIN_GO.read_text(encoding="utf-8")
    js = APP_JS.read_text(encoding="utf-8")

    item_keys = all_keys(game["items"])
    monster_keys = all_keys(game["monsters"])
    server_keys = {key for server in game["servers"].values() for key in server.keys()}
    effect_spec_keys = all_keys(game["effectSpecs"].values())
    ability_fields = all_keys(abilities.get("items", {}).values())
    monster_detail_fields = all_keys(monster_details.get("monsters", {}).values())

    item_go = json_tags(go_source, "Item")
    item_patch_go = json_tags(go_source, "itemAbilityPatch")
    monster_go = json_tags(go_source, "Monster")
    monster_patch_go = json_tags(go_source, "monsterDetailPatch")

    checks = {
        "item_unclassified": sorted(item_keys - ITEM_PRESENTATION.keys()),
        "monster_unclassified": sorted(monster_keys - MONSTER_PRESENTATION.keys()),
        "server_unclassified": sorted(server_keys - SERVER_PRESENTATION.keys()),
        "item_missing_go_fields": sorted(item_keys - item_go),
        "monster_missing_go_fields": sorted(monster_keys - monster_go),
        "server_missing_go_fields": sorted(server_keys - json_tags(go_source, "ServerData")),
        "effect_spec_missing_go_fields": sorted(effect_spec_keys - json_tags(go_source, "ItemEffectSpec")),
        "ability_supplement_unclassified_fields": sorted(ability_fields - ITEM_SUPPLEMENT_PRESENTATION.keys()),
        "ability_supplement_missing_item_fields": sorted(ability_fields - item_go),
        "ability_supplement_missing_patch_fields": sorted(ability_fields - item_patch_go),
        "monster_supplement_unclassified_fields": sorted(monster_detail_fields - MONSTER_SUPPLEMENT_PRESENTATION.keys()),
        "monster_supplement_missing_monster_fields": sorted(monster_detail_fields - monster_go),
        "monster_supplement_missing_patch_fields": sorted(monster_detail_fields - monster_patch_go),
    }

    effect_specs = {int(key): value for key, value in game["effectSpecs"].items()}
    base_items_by_id = {str(item["id"]): item for item in game["items"]}
    supplement_ids_missing = sorted(set(abilities.get("items", {})) - set(base_items_by_id), key=int)
    monster_ids = {str(monster["id"]) for monster in game["monsters"]}
    monster_supplement_ids_missing = sorted(set(monster_details.get("monsters", {})) - monster_ids, key=int)

    item_option_types = collections.Counter(option["type"] for item in game["items"] for option in item.get("options", []))
    for item_id, patch in abilities.get("items", {}).items():
        if "options" in patch and "options" not in base_items_by_id[item_id]:
            item_option_types.update(option["type"] for option in patch["options"])
    unknown_item_effects = {key: value for key, value in sorted(item_option_types.items()) if key not in effect_specs}
    zero_item_options = sum(1 for item in game["items"] for option in item.get("options", []) if option.get("value") == 0)
    zero_item_options += sum(1 for patch in abilities.get("items", {}).values() for option in patch.get("options", []) if option.get("value") == 0)
    ability_patch_field_counts = collections.Counter(key for patch in abilities.get("items", {}).values() for key in patch)
    monster_patch_field_counts = collections.Counter(key for patch in monster_details.get("monsters", {}).values() for key in patch)
    unknown_make_skill_codes = sorted({patch.get("makeSkill") for patch in abilities.get("items", {}).values() if patch.get("makeSkill")} - {1, 2, 3, 4, 5})
    unknown_use_map_codes = sorted({patch.get("useMapType") for patch in abilities.get("items", {}).values() if patch.get("useMapType")} - {1, 2, 3, 4, 5, 6, 7})
    unknown_guild_use_codes = sorted({patch.get("guildUse") for patch in abilities.get("items", {}).values() if patch.get("guildUse")} - {1, 2})

    set_rows = []
    exact_duplicate_set_rows = 0
    sets_with_exact_duplicate_rows = 0
    active_states = collections.Counter()
    thresholds = collections.Counter()
    active_thresholds = collections.Counter()
    set_option_types = collections.Counter()
    max_effect_lines_same_threshold = 0
    max_rows_same_threshold = 0
    for set_id, set_data in sets["sets"].items():
        row_keys = [json.dumps(row, ensure_ascii=False, sort_keys=True) for row in set_data.get("effects", [])]
        row_counts = collections.Counter(row_keys)
        duplicates_here = sum(count - 1 for count in row_counts.values() if count > 1)
        if duplicates_here:
            sets_with_exact_duplicate_rows += 1
            exact_duplicate_set_rows += duplicates_here
        threshold_rows = collections.Counter()
        threshold_lines = collections.Counter()
        for position, row in enumerate(set_data.get("effects", [])):
            required = row["required"]
            thresholds[required] += 1
            threshold_rows[required] += 1
            threshold_lines[required] += len(row.get("options", [])) + (1 if row.get("active") else 0)
            for option in row.get("options", []):
                set_option_types[option["type"]] += 1
            if row.get("active"):
                active_states[row["active"]["state"]] += 1
                active_thresholds[required] += 1
            set_rows.append((set_id, position, row))
        max_rows_same_threshold = max(max_rows_same_threshold, max(threshold_rows.values(), default=0))
        max_effect_lines_same_threshold = max(max_effect_lines_same_threshold, max(threshold_lines.values(), default=0))

    unknown_set_effects = {key: value for key, value in sorted(set_option_types.items()) if key not in effect_specs}
    unknown_active_states = {key: value for key, value in sorted(active_states.items()) if key not in {1, 2, 3}}
    embedded_sets = game["itemSets"]
    embedded_member_set_ids = {key for key, value in embedded_sets.items() if value.get("items")}
    supplement_set_ids = set(sets["sets"])
    sets_effects_without_published_members = sorted(supplement_set_ids - embedded_member_set_ids, key=int)
    member_sets_without_effects = sorted(embedded_member_set_ids - supplement_set_ids, key=int)
    threshold_counts_per_set = [len({row["required"] for row in data.get("effects", [])}) for data in sets["sets"].values()]

    card_slot_types = collections.Counter(slot for item in game["items"] for slot in item.get("cardSlots", []))
    known_slot_types = {"A", "B", "AB", "O", "M", "MI++++"}

    recipe_ids = set(recipes.get("recipes", {}))
    recipe_ids_missing_from_game = sorted(recipe_ids - set(base_items_by_id), key=int)
    recipe_material_ids = [str(row["itemId"]) for materials in recipes.get("recipes", {}).values() for row in materials]
    missing_recipe_material_ids = sorted({value for value in recipe_material_ids if value not in base_items_by_id}, key=int)

    checks.update({
        "unknown_item_effect_types": unknown_item_effects,
        "unknown_set_effect_types": unknown_set_effects,
        "unknown_active_states": unknown_active_states,
        "unknown_card_slot_types": sorted(set(card_slot_types) - known_slot_types),
        "ability_supplement_ids_missing_from_game_data": supplement_ids_missing,
        "monster_supplement_ids_missing_from_game_data": monster_supplement_ids_missing,
        "recipe_ids_missing_from_game_data": recipe_ids_missing_from_game,
        "recipe_material_ids_missing_from_game_data": missing_recipe_material_ids,
        "recipe_missing_material_fallback_absent": bool(missing_recipe_material_ids) and "Неизвестный предмет (ID %d)" not in go_source,
        "unknown_make_skill_codes": unknown_make_skill_codes,
        "unknown_use_map_codes": unknown_use_map_codes,
        "unknown_guild_use_codes": unknown_guild_use_codes,
        "ability_supplement_not_merged": "mergeItemAbilitySupplement()" not in go_source or "assets/item_abilities.json.gz" not in go_source,
        "monster_supplement_not_merged": "mergeMonsterDetailSupplement()" not in go_source or "assets/monster_details.json.gz" not in go_source,
        "set_supplement_not_merged": "mergeSetSupplement()" not in go_source or "assets/set_effects.json.gz" not in go_source,
        "recipe_supplement_not_merged": "mergeRecipeSupplement()" not in go_source or "assets/item_recipes.json.gz" not in go_source,
        "frontend_has_set_slice_limit": bool(re.search(r"set\.(?:effects|items)\s*\.slice\(0,", js)),
        "frontend_uses_generic_properties_array": "const properties =" in js,
        "monster_id_in_suggestion_preview": bool(re.search(r"record\.category,\s*record\.typeName.*?ID \$\{record\.id\}", js, re.S)),
        "frontend_loses_bonus_known_flag": bool(re.search(r"data\.bonuses\s*\|\|\s*\[\]\)\.map\(row\s*=>\s*\[row\.name,\s*row\.value\]\)", js)),
    })

    fatal_keys = (
        "item_unclassified", "monster_unclassified", "server_unclassified", "item_missing_go_fields",
        "monster_missing_go_fields", "server_missing_go_fields", "effect_spec_missing_go_fields",
        "ability_supplement_unclassified_fields", "ability_supplement_missing_item_fields",
        "ability_supplement_missing_patch_fields", "monster_supplement_unclassified_fields",
        "monster_supplement_missing_monster_fields", "monster_supplement_missing_patch_fields",
        "unknown_set_effect_types", "unknown_active_states", "unknown_card_slot_types",
        "ability_supplement_ids_missing_from_game_data", "monster_supplement_ids_missing_from_game_data",
        "recipe_ids_missing_from_game_data",
        "unknown_make_skill_codes", "unknown_use_map_codes", "unknown_guild_use_codes",
    )
    fatal = [name for name in fatal_keys if checks[name]]
    for name in ("ability_supplement_not_merged", "monster_supplement_not_merged", "set_supplement_not_merged", "recipe_supplement_not_merged", "recipe_missing_material_fallback_absent"):
        if checks[name]:
            fatal.append(name)
    if checks["frontend_has_set_slice_limit"] or checks["frontend_uses_generic_properties_array"] or checks["monster_id_in_suggestion_preview"] or checks["frontend_loses_bonus_known_flag"]:
        fatal.append("frontend_loss_or_privacy_rule")

    return {
        "gameDataSha256": hashlib.sha256(GAME_DATA.read_bytes()).hexdigest(),
        "setEffectsSha256": hashlib.sha256(SET_EFFECTS.read_bytes()).hexdigest(),
        "itemAbilitiesSha256": hashlib.sha256(ITEM_ABILITIES.read_bytes()).hexdigest(),
        "itemRecipesSha256": hashlib.sha256(ITEM_RECIPES.read_bytes()).hexdigest(),
        "monsterDetailsSha256": hashlib.sha256(MONSTER_DETAILS.read_bytes()).hexdigest(),
        "items": len(game["items"]), "monsters": len(game["monsters"]), "setsInEmbedded": len(embedded_sets),
        "setsWithPublishedMembers": len(embedded_member_set_ids),
        "setsWithEffectDefinitions": len(sets["sets"]), "setEffectRows": len(set_rows),
        "setThresholds": sorted(thresholds), "setThresholdRowCounts": dict(sorted(thresholds.items())),
        "activeEffectRows": sum(active_states.values()), "activeThresholdCounts": dict(sorted(active_thresholds.items())),
        "setsWithFivePieceEffects": sum(1 for data in sets["sets"].values() if any(row["required"] == 5 for row in data.get("effects", []))),
        "fivePieceActiveEffects": active_thresholds.get(5, 0),
        "maxDistinctThresholdsPerSet": max(threshold_counts_per_set, default=0),
        "maxRowsAtSameThreshold": max_rows_same_threshold,
        "maxEffectLinesAtSameThreshold": max_effect_lines_same_threshold,
        "exactDuplicateSetRows": exact_duplicate_set_rows,
        "setsWithExactDuplicateRows": sets_with_exact_duplicate_rows,
        "setsEffectDefinitionsWithoutPublishedMembers": len(sets_effects_without_published_members),
        "setEffectDefinitionIDsWithoutPublishedMembers": sets_effects_without_published_members,
        "publishedMemberSetsWithoutEffects": len(member_sets_without_effects),
        "publishedMemberSetIDsWithoutEffects": member_sets_without_effects,
        "itemOptionRows": sum(item_option_types.values()), "explicitZeroItemOptions": zero_item_options,
        "itemAbilitySupplementItems": len(abilities.get("items", {})),
        "itemAbilitySupplementFieldCounts": dict(sorted(ability_patch_field_counts.items())),
        "monsterSupplementMonsters": len(monster_details.get("monsters", {})),
        "monsterSupplementFieldCounts": dict(sorted(monster_patch_field_counts.items())),
        "recipeRows": len(recipes.get("recipes", {})),
        "recipeMaterialLinks": len(recipe_material_ids),
        "maxRecipeMaterials": max((len(rows) for rows in recipes.get("recipes", {}).values()), default=0),
        "restoredAbilityDescriptions": ability_patch_field_counts.get("abilityDescription", 0),
        "restoredEffectDurations": ability_patch_field_counts.get("effectDurationMs", 0),
        "restoredItemLimitUsageRules": ability_patch_field_counts.get("useMapType", 0),
        "restoredProfessionRules": ability_patch_field_counts.get("makeSkill", 0),
        "restoredGuildRules": ability_patch_field_counts.get("guildUse", 0),
        "preservedRawAbilityConflicts": len(abilities.get("conflictsPreserved", [])),
        "unknownItemEffectTypes": unknown_item_effects,
        "cardSlotTypes": dict(sorted(card_slot_types.items())),
        "checks": checks,
        "fatal": fatal,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true", help="Print machine-readable JSON")
    args = parser.parse_args()
    result = audit()
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(f"items={result['items']} monsters={result['monsters']} sets={result['setsInEmbedded']}")
        print(f"set rows={result['setEffectRows']} thresholds={result['setThresholds']} active={result['activeEffectRows']}")
        print(f"5-piece sets={result['setsWithFivePieceEffects']} active@5={result['fivePieceActiveEffects']}")
        print(f"recipes={result['recipeRows']} materials={result['recipeMaterialLinks']}")
        print(f"unknown item option enums={result['unknownItemEffectTypes']}")
        print(f"fatal={result['fatal']}")
    if result["fatal"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
