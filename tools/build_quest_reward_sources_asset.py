"""Build confirmed quest-completion sources for title items and recipes."""

from __future__ import annotations

import argparse
import gzip
import json
import re
from pathlib import Path

QUEST_RE = re.compile(r"^\s*quest\s+(\d+)\s*$")
TITLE_RE = re.compile(r"^\s*title\s+(\d+)\s*$")
ITEM_RE = re.compile(r"^\s*item\s+(\d+)\s+(\d+)\s*$")
QUEST_FILES = (
    "quest_main.txt",
    "quest_extra.txt",
    "quest_scroll.txt",
    "quest_ivent.txt",
)


def load_json_gzip(path: Path) -> dict:
    with gzip.open(path, "rt", encoding="utf-8") as source:
        return json.load(source)


def load_quest_names(path: Path) -> dict[int, str]:
    names: dict[int, str] = {}
    for raw in path.read_text(encoding="utf-16").splitlines():
        if not raw.strip():
            continue
        key, separator, value = raw.partition("\t")
        if not separator:
            continue
        try:
            index = int(key.strip())
        except ValueError:
            continue
        name = value.strip().strip('"').strip()
        if index > 0 and name:
            names[index] = name
    return names


def target_item_ids(item_abilities: Path, item_recipes: Path) -> set[int]:
    abilities = load_json_gzip(item_abilities)
    recipes = load_json_gzip(item_recipes)
    title_items = {
        int(item_id)
        for item_id, patch in abilities.get("items", {}).items()
        if int(patch.get("titleIndex", 0) or 0) > 0
    }
    recipe_items = {int(item_id) for item_id in recipes.get("recipes", {})}
    return title_items | recipe_items


def parse_quest_file(
    path: Path,
    quest_names: dict[int, str],
    targets: set[int],
) -> list[dict[str, int | str]]:
    rewards: list[dict[str, int | str]] = []
    lines = path.read_text(encoding="cp1251", errors="replace").splitlines()
    quest_id = 0
    quest_title_index = 0
    depth = 0
    started = False
    section = ""
    section_depth = 0
    pending_section = ""

    for line in lines:
        if not quest_id:
            match = QUEST_RE.match(line)
            if not match:
                continue
            quest_id = int(match.group(1))
            quest_title_index = 0
            depth = 0
            started = False
            section = ""
            section_depth = 0
            pending_section = ""
            continue

        stripped = line.strip()
        if depth == 1:
            title_match = TITLE_RE.match(line)
            if title_match and quest_title_index == 0:
                quest_title_index = int(title_match.group(1))
            if stripped in {"default", "select"}:
                pending_section = stripped

        opens = line.count("{")
        closes = line.count("}")
        if opens:
            started = True
        if pending_section and opens and depth == 1:
            section = pending_section
            section_depth = depth + 1
            pending_section = ""

        item_match = ITEM_RE.match(line)
        if item_match and section and depth == section_depth and quest_title_index > 0:
            item_id = int(item_match.group(1))
            quantity = max(1, int(item_match.group(2)))
            if item_id in targets:
                quest_name = quest_names.get(quest_title_index, "").strip()
                if not quest_name:
                    raise ValueError(
                        f"missing quest localization {quest_title_index} "
                        f"for quest {quest_id}"
                    )
                rewards.append(
                    {
                        "itemId": item_id,
                        "questId": quest_id,
                        "questTitleIndex": quest_title_index,
                        "quest": quest_name,
                        "rewardType": section,
                        "quantity": quantity,
                    }
                )

        depth += opens - closes
        if section and depth < section_depth:
            section = ""
            section_depth = 0
        if started and depth == 0:
            quest_id = 0
            quest_title_index = 0
            pending_section = ""

    return rewards


def build(
    quest_root: Path,
    quest_names_path: Path,
    item_abilities: Path,
    item_recipes: Path,
) -> dict:
    names = load_quest_names(quest_names_path)
    targets = target_item_ids(item_abilities, item_recipes)
    rewards: list[dict[str, int | str]] = []
    for filename in QUEST_FILES:
        rewards.extend(parse_quest_file(quest_root / filename, names, targets))

    rewards.sort(
        key=lambda row: (
            int(row["itemId"]),
            int(row["questId"]),
            str(row["rewardType"]),
        )
    )
    seen: set[tuple[int, int, str]] = set()
    for row in rewards:
        key = (
            int(row["itemId"]),
            int(row["questId"]),
            str(row["rewardType"]),
        )
        if key in seen:
            raise ValueError(f"duplicate quest reward relation: {key}")
        seen.add(key)
    return {"schemaVersion": 1, "rewards": rewards}


def write_gzip_json(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(
        data,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=False,
    ).encode("utf-8")
    with (
        path.open("wb") as output,
        gzip.GzipFile(
            filename="",
            mode="wb",
            fileobj=output,
            mtime=0,
            compresslevel=9,
        ) as compressed,
    ):
        compressed.write(payload)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--quest-root", type=Path, required=True)
    parser.add_argument("--quest-names", type=Path, required=True)
    parser.add_argument("--item-abilities", type=Path, required=True)
    parser.add_argument("--item-recipes", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    data = build(
        args.quest_root,
        args.quest_names,
        args.item_abilities,
        args.item_recipes,
    )
    write_gzip_json(args.output, data)
    print(f"quest reward relations={len(data['rewards'])} output={args.output}")


if __name__ == "__main__":
    main()
