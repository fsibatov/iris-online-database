#!/usr/bin/env python3
"""Build the additive chest-content projection from Iris Online item_change.txt.

The original server tables are intentionally not bundled with the application.
Only profiles whose source item is confirmed as an item-change container are
retained. The confirmation comes from the additive item-ability projection:
kindOf=3, eventType=3 and changeIndex equal to the source item ID. This includes
quest-reward boxes that are not catalogued under the UI category "Сундук".
Source row order and the original item/count/enhanced/changerate values are
preserved; probability interpretation stays in the Go runtime so it can be
tested together with the API contract.
"""

from __future__ import annotations

import argparse
import gzip
import json
from pathlib import Path


def _int(value: str, default: int = 0) -> int:
    try:
        return int(value.strip())
    except (AttributeError, ValueError):
        return default


def load_items(game_data: Path) -> dict[int, dict]:
    with gzip.open(game_data, "rt", encoding="utf-8") as source:
        data = json.load(source)
    return {int(item["id"]): item for item in data.get("items", [])}


def load_container_ids(item_abilities: Path) -> set[int]:
    with gzip.open(item_abilities, "rt", encoding="utf-8") as source:
        data = json.load(source)
    result: set[int] = set()
    for key, patch in data.get("items", {}).items():
        item_id = _int(key)
        if item_id <= 0:
            continue
        if (
            patch.get("kindOf") == 3
            and patch.get("eventType") == 3
            and patch.get("changeIndex") == item_id
        ):
            result.add(item_id)
    return result


def parse_profiles(
    path: Path, items: dict[int, dict], container_ids: set[int]
) -> dict[str, dict]:
    profiles: dict[str, dict] = {}
    current: dict[str, list[str] | int] | None = None

    def flush() -> None:
        nonlocal current
        if not current:
            return
        source_id = int(current.get("index", 0))
        source_item = items.get(source_id)
        if not source_item:
            current = None
            return
        if source_id not in container_ids:
            current = None
            return

        item_ids = [_int(value) for value in current.get("item", [])]
        counts = [_int(value, 1) for value in current.get("count", [])]
        enhanced = [_int(value) for value in current.get("enhanced", [])]
        thresholds = [_int(value) for value in current.get("changerate", [])]
        row_count = min(len(item_ids), len(counts), len(enhanced), len(thresholds))
        rows: list[dict[str, int]] = []
        for position in range(row_count):
            item_id = item_ids[position]
            threshold = thresholds[position]
            if item_id <= 0 or threshold <= 0:
                continue
            rows.append(
                {
                    "itemId": item_id,
                    "quantity": max(1, counts[position]),
                    "enhanced": max(0, enhanced[position]),
                    "threshold": threshold,
                    "position": position + 1,
                }
            )

        profiles[str(source_id)] = {
            "drawCount": max(0, _int(str((current.get("rate") or ["0"])[0]))),
            "rows": rows,
        }
        current = None

    for raw in path.read_text(encoding="utf-8", errors="replace").splitlines():
        parts = raw.split("\t")
        key = parts[0].strip().lower() if parts else ""
        values = [value.strip() for value in parts[1:] if value.strip()]
        if key == "index":
            flush()
            current = {"index": _int(values[0]) if values else 0}
        elif current is not None and key in {
            "rate",
            "item",
            "count",
            "enhanced",
            "changerate",
        }:
            current[key] = values
    flush()
    return profiles


def build(game_data: Path, item_abilities: Path, kiss: Path, original: Path) -> dict:
    items = load_items(game_data)
    container_ids = load_container_ids(item_abilities)
    return {
        "schemaVersion": 1,
        "servers": {
            "kiss": {"profiles": parse_profiles(kiss, items, container_ids)},
            "original": {"profiles": parse_profiles(original, items, container_ids)},
        },
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--game-data", type=Path, required=True)
    parser.add_argument("--item-abilities", type=Path, required=True)
    parser.add_argument("--kiss-item-change", type=Path, required=True)
    parser.add_argument("--original-item-change", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    data = build(
        args.game_data,
        args.item_abilities,
        args.kiss_item_change,
        args.original_item_change,
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

    for server, payload_server in data["servers"].items():
        profiles = payload_server["profiles"]
        rows = sum(len(profile["rows"]) for profile in profiles.values())
        print(f"{server}: chest_profiles={len(profiles)} rows={rows}")
    print(f"output={args.output}")


if __name__ == "__main__":
    main()
