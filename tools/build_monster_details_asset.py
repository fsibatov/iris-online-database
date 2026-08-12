#!/usr/bin/env python3
"""Build a lossless additive projection of monsterlist fields omitted by game_data.

The asset does not reinterpret opaque client/server fields. Fields without a
confirmed player-facing meaning are exposed only in technical details.
"""

from __future__ import annotations

import argparse
import gzip
import json
from pathlib import Path

FIELDS = {
    "nameIndex": 1,
    "noteIndex": 3,
    "nameHeight": 4,
    "sourceFlag": 7,
    "effectScale": 21,
    "freeMoveRange": 23,
    "actionStopRatio": 24,
    "actionWalkRatio": 25,
    "actionRunRatio": 26,
    "actionStopTime": 27,
    "changeMonsterCheck": 31,
    "followTime": 32,
    "escapeType": 34,
    "escapePercent": 35,
    "recoveryTime": 37,
}
FLOAT_FIELDS = {"effectScale"}


def parse_number(value: str, as_float: bool):
    try:
        return float(value) if as_float else int(value)
    except (TypeError, ValueError):
        return None


def build(monster_list: Path) -> dict:
    monsters: dict[str, dict] = {}
    for raw in monster_list.read_text(encoding="cp1251", errors="replace").splitlines():
        if not raw or raw.startswith("//"):
            continue
        parts = raw.split("\t")
        try:
            monster_id = int(parts[0])
        except (IndexError, ValueError):
            continue
        row = {}
        for field, position in FIELDS.items():
            if position >= len(parts):
                continue
            value = parse_number(parts[position], field in FLOAT_FIELDS)
            if value is not None:
                row[field] = value
        monsters[str(monster_id)] = row
    return {"schemaVersion": 1, "monsters": monsters}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--monster-list", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = build(args.monster_list)
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
    print(f"monsters={len(data['monsters'])} output={args.output}")


if __name__ == "__main__":
    main()
