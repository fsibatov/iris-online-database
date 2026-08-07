#!/usr/bin/env python3
"""Build the additive set-effect projection from original Iris Online resources.

The source resources are intentionally NOT bundled with the public/source release.
Pass explicit paths when regenerating the asset.
"""
from __future__ import annotations

import argparse
import gzip
import json
from pathlib import Path


def read_indexed_text(path: Path) -> dict[int, str]:
    result: dict[int, str] = {}
    for raw in path.read_text(encoding="utf-16", errors="strict").splitlines():
        if not raw or raw.startswith("//"):
            continue
        parts = raw.split("\t", 1)
        if len(parts) != 2:
            continue
        try:
            index = int(parts[0].strip())
        except ValueError:
            continue
        value = parts[1].strip().strip('"')
        result[index] = value
    return result


def read_active_skills(path: Path) -> dict[int, dict]:
    result: dict[int, dict] = {}
    for raw in path.read_text(encoding="cp1251", errors="replace").splitlines():
        if not raw or raw.startswith("//"):
            continue
        parts = raw.split("\t")
        if len(parts) < 5:
            continue
        try:
            index = int(parts[0])
            tooltip_index = int(parts[1])
            state = int(parts[3])
            chance_raw = int(parts[4])
        except ValueError:
            continue
        result[index] = {
            "id": index,
            "tooltipIndex": tooltip_index,
            "state": state,
            "chance": chance_raw / 10_000.0,
        }
    return result


def build(item_set: Path, item_names: Path, item_tooltips: Path, active_skills: Path) -> dict:
    names = read_indexed_text(item_names)
    tooltips = read_indexed_text(item_tooltips)
    active = read_active_skills(active_skills)
    sets: dict[str, dict] = {}

    for raw in item_set.read_text(encoding="cp1251", errors="replace").splitlines():
        if not raw or raw.startswith("//"):
            continue
        parts = raw.split("\t")
        if len(parts) < 8:
            continue
        try:
            set_id = int(parts[0])
            required = int(parts[1])
            name_index = int(parts[2])
            option1_type = int(parts[3])
            option1_value = int(parts[4])
            option2_type = int(parts[5])
            option2_value = int(parts[6])
            active_id = int(parts[7])
        except ValueError:
            continue

        target = sets.setdefault(str(set_id), {"name": names.get(name_index, ""), "effects": []})
        if not target["name"] and names.get(name_index):
            target["name"] = names[name_index]
        row = {"required": required, "options": []}
        if option1_type:
            row["options"].append({"type": option1_type, "value": option1_value})
        if option2_type:
            row["options"].append({"type": option2_type, "value": option2_value})
        if active_id:
            source = active.get(active_id)
            if source is None:
                raise SystemExit(f"Active set effect {active_id} is missing from the active-skill resource")
            active_row = dict(source)
            active_row["text"] = tooltips.get(source["tooltipIndex"], "")
            row["active"] = active_row
        target["effects"].append(row)

    return {"schemaVersion": 1, "sets": sets}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--item-set", type=Path, required=True)
    parser.add_argument("--item-names", type=Path, required=True)
    parser.add_argument("--item-tooltips", type=Path, required=True)
    parser.add_argument("--active-skills", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = build(args.item_set, args.item_names, args.item_tooltips, args.active_skills)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(data, ensure_ascii=False, separators=(",", ":"), sort_keys=False).encode("utf-8")
    # mtime=0 makes the gzip artifact reproducible.
    with args.output.open("wb") as output:
        with gzip.GzipFile(filename="", mode="wb", fileobj=output, mtime=0, compresslevel=9) as gz:
            gz.write(payload)
    print(f"sets={len(data['sets'])} rows={sum(len(s['effects']) for s in data['sets'].values())} output={args.output}")


if __name__ == "__main__":
    main()
