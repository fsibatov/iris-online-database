"""Build lossless item-enhancement profiles from item_enhanced.txt.

Only the raw enhancement table is projected here. Player-facing calculations are
performed by the application using the same core formulas visible in
ItemTipWindow_Set.cpp for default attack, defence and healing enhancement types.
"""

from __future__ import annotations

import argparse
import gzip
import json
from pathlib import Path

LEVELS = 10


def _number(value: str) -> int | float:
    number = float(value)
    return int(number) if number.is_integer() else number


def build(path: Path) -> dict:
    profiles: dict[str, list[dict]] = {}
    current: str | None = None
    for raw in path.read_text(encoding="cp1251", errors="strict").splitlines():
        parts = raw.split("\t")
        if parts and parts[0] == "index":
            if len(parts) < 2 or not parts[1].strip().isdigit():
                raise ValueError(f"invalid enhancement index line: {raw!r}")
            current = str(int(parts[1]))
            if current in profiles:
                raise ValueError(f"duplicate enhancement profile: {current}")
            profiles[current] = []
            continue
        if parts and parts[0] == "end":
            current = None
            continue
        if current is None:
            continue
        values = [part.strip() for part in parts if part.strip()]
        if not values:
            continue
        if len(values) != LEVELS + 2:
            raise ValueError(
                f"profile {current}: expected {LEVELS + 2} values, got {len(values)}"
            )
        equip = int(values[0])
        option_type = int(values[1])
        levels = [_number(value) for value in values[2:]]
        profiles[current].append(
            {"equip": equip, "type": option_type, "values": levels}
        )
    if not profiles:
        raise ValueError("item_enhanced.txt did not contain enhancement profiles")
    return {"schemaVersion": 1, "levels": LEVELS, "profiles": profiles}


def write(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(data, ensure_ascii=False, separators=(",", ":")).encode(
        "utf-8"
    )
    with (
        path.open("wb") as output,
        gzip.GzipFile(
            filename="", mode="wb", fileobj=output, mtime=0, compresslevel=9
        ) as gz,
    ):
        gz.write(payload)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--item-enhanced", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = build(args.item_enhanced)
    write(args.output, data)
    rows = sum(len(profile) for profile in data["profiles"].values())
    print(f"profiles={len(data['profiles'])} rows={rows} output={args.output}")


if __name__ == "__main__":
    main()
