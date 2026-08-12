"""Build the additive recipe-material projection from original Iris Online resources.

The original resource files are intentionally not bundled with the application.
The generated asset preserves recipe row order and ingredient order and contains
only IDs/counts; names are resolved from the embedded item database at runtime.
"""

from __future__ import annotations

import argparse
import gzip
import json
from pathlib import Path


def safe_int(value: str) -> int | None:
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def build(item_mixed: Path) -> dict:
    recipes: dict[str, list[dict[str, int]]] = {}
    for raw in item_mixed.read_text(encoding="cp1251", errors="replace").splitlines():
        if not raw or raw.startswith("//"):
            continue
        parts = raw.split("\t")
        if len(parts) < 25 or parts[0].strip().lower() != "mix":
            continue
        recipe_id = safe_int(parts[7]) if len(parts) > 7 else None
        if not recipe_id:
            continue
        ingredients: list[dict[str, int]] = []

        for position in range(24, 42, 3):
            if position >= len(parts):
                break
            item_id = (
                safe_int(parts[position].strip()) if parts[position].strip() else 0
            )
            quantity = (
                safe_int(parts[position + 1].strip())
                if position + 1 < len(parts) and parts[position + 1].strip()
                else 0
            )
            if item_id:
                ingredients.append(
                    {"itemId": item_id, "quantity": max(1, quantity or 1)}
                )
        recipes[str(recipe_id)] = ingredients
    return {"schemaVersion": 1, "recipes": recipes}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--item-mixed", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = build(args.item_mixed)
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
    print(
        f"recipes={len(data['recipes'])} ingredients={sum(len(rows) for rows in data['recipes'].values())} output={args.output}"
    )


if __name__ == "__main__":
    main()
