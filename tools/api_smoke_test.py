#!/usr/bin/env python3
"""Reproducible API smoke test for Iris Online."""
from __future__ import annotations

import argparse
from smoke_common import RunningApp, json_request, require_binary


def assert_status(base: str, path: str, expected: int = 200, **kwargs):
    status, payload = json_request(base, path, **kwargs)
    if status != expected:
        raise AssertionError(f"{path}: status {status}, expected {expected}; payload={payload!r}")
    return payload


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    binary = require_binary(args.binary)

    with RunningApp(binary, ["-no-browser"]) as app:
        app.wait_ready()
        health = assert_status(app.base_url, "/api/health")
        assert health["application"] == "iris-online-database"
        assert health["version"] == "1.1.0"

        search = assert_status(app.base_url, "/api/search?q=%D0%B3%D0%BD%D0%B5%D0%B2%20%D0%BF%D1%80%D0%B5%D0%B4%D0%BA%D0%BE%D0%B2")
        assert search["items"] or search["monsters"], "word-form search returned nothing"

        by_id = assert_status(app.base_url, "/api/items?q=80592&page=1&pageSize=24")
        assert any(row["id"] == 80592 for row in by_id["items"])

        empty = assert_status(app.base_url, "/api/items?q=__definitely_missing__&page=1&pageSize=24")
        assert empty["total"] == 0
        assert_status(app.base_url, "/api/items/999999999", expected=404)

        filtered = assert_status(app.base_url, "/api/items?category=%D0%9E%D1%80%D1%83%D0%B6%D0%B8%D0%B5%2F%D1%89%D0%B8%D1%82&page=1&pageSize=24&sort=level")
        assert filtered["page"] == 1 and len(filtered["items"]) <= 24

        recipes = assert_status(app.base_url, "/api/recipes?page=1&pageSize=24&sort=name")
        assert recipes["total"] == 1207, recipes["total"]
        assert len(recipes["recipes"]) <= 24 and recipes["recipes"], "recipe catalog is empty"
        assert recipes["recipes"][0].get("materials"), "recipe row has no materials"
        food_recipes = assert_status(app.base_url, "/api/recipes?type=%D0%A0%D0%B5%D1%86%D0%B5%D0%BF%D1%82%20%D0%B5%D0%B4%D1%8B&page=1&pageSize=24&sort=mastery")
        assert food_recipes["total"] == 194, food_recipes["total"]
        mastery_values = [int(row.get("masteryLevel", 0)) for row in food_recipes["recipes"]]
        assert mastery_values == sorted(mastery_values), mastery_values
        reference_recipe = assert_status(app.base_url, "/api/recipes?q=891219&page=1&pageSize=24&sort=mastery")
        reference_rows = [row for row in reference_recipe["recipes"] if int(row.get("id", 0)) == 891219]
        assert reference_rows, "reference recipe 891219 is missing"
        assert int(reference_rows[0].get("makeSkill", 0)) == 2, reference_rows[0]
        assert int(reference_rows[0].get("masteryLevel", -1)) == 20, reference_rows[0]
        assert int(reference_rows[0].get("level", -1)) != 20, reference_rows[0]
        sourced_recipes = assert_status(app.base_url, "/api/recipes?knownSource=1&server=kiss&page=1&pageSize=24&sort=name")
        assert sourced_recipes["total"] > 0, "known-source recipe filter returned nothing"
        assert all(row.get("sourceCount", 0) > 0 and row.get("sourcePreview", {}).get("name") for row in sourced_recipes["recipes"]), "recipe source preview missing"
        paged = assert_status(app.base_url, "/api/monsters?page=2&pageSize=24&sort=level")
        assert paged["page"] == 2 and len(paged["monsters"]) <= 24
        kiss_catalog = assert_status(app.base_url, "/api/monsters?server=kiss&page=1&pageSize=8")
        original_catalog = assert_status(app.base_url, "/api/monsters?server=original&page=1&pageSize=8")
        assert kiss_catalog["total"] == 677, kiss_catalog["total"]
        assert original_catalog["total"] == 609, original_catalog["total"]
        assert_status(app.base_url, "/api/monsters/11026?server=original")
        assert_status(app.base_url, "/api/monsters/11026?server=kiss", expected=404)
        assert_status(app.base_url, "/api/monsters/1122?server=kiss")
        assert_status(app.base_url, "/api/monsters/1122?server=original", expected=404)
        assert_status(app.base_url, "/api/monsters/253?server=kiss", expected=404)
        assert_status(app.base_url, "/api/monsters/253?server=original", expected=404)
        bounded = assert_status(app.base_url, "/api/items?page=-50&pageSize=999999")
        assert bounded["page"] == 1 and bounded["pageSize"] == 48 and len(bounded["items"]) <= 48
        assert_status(app.base_url, "/api/items?sort=unknown-enum", expected=400)
        assert_status(app.base_url, "/api/items/not-a-number", expected=400)
        assert_status(app.base_url, "/api/monsters/not-a-number", expected=400)
        assert_status(app.base_url, "/api/items", method="POST", expected=405)

        keys: list[str] = []
        page = 1
        while len(keys) < 620:
            data = assert_status(app.base_url, f"/api/items?page={page}&pageSize=48&sort=name")
            keys.extend(f"item:{row['id']}" for row in data["items"])
            if page >= data["pages"]:
                break
            page += 1
        assert len(keys) >= 620, f"only {len(keys)} item IDs available"
        favorites = assert_status(
            app.base_url,
            "/api/favorites",
            method="POST",
            payload={"keys": keys[:620], "server": "kiss", "page": 13, "pageSize": 50},
        )
        assert favorites["total"] == 620
        assert favorites["pages"] == 13
        assert len(favorites["rows"]) == 20

        chest_item = assert_status(app.base_url, "/api/items/101402?server=kiss")
        chest_source = next((row for row in chest_item["drops"] if row.get("source") == "Сундук" and row.get("containerId") == 808094), None)
        assert chest_source is not None, "labyrinth cloth chest source is missing"
        assert chest_source.get("chanceKnown") is True, chest_source
        assert abs(chest_source["itemBaseChance"] - 15.204) < 1e-9, chest_source

        chest = assert_status(app.base_url, "/api/items/808094?server=kiss")
        silk_hat = next((row for row in (chest.get("chest") or {}).get("items", []) if row.get("itemId") == 101402), None)
        assert silk_hat is not None and silk_hat.get("chanceKnown") is True and abs(silk_hat["chance"] - 15.204) < 1e-9, silk_hat

        quest_box = assert_status(app.base_url, "/api/items/873063?server=kiss")
        assert (quest_box.get("chest") or {}).get("items"), "non-category item-change box was filtered out"

        anomalous_box = assert_status(app.base_url, "/api/items/873079?server=kiss")
        anomalous_items = (anomalous_box.get("chest") or {}).get("items", [])
        assert anomalous_items, "anomalous box contents are missing"
        assert all(row.get("chanceKnown") is False and "chance" not in row for row in anomalous_items), anomalous_items

        world_item = assert_status(app.base_url, "/api/items/1055001?server=kiss")
        world_source = next((row for row in world_item["drops"] if row.get("source") == "Мировое выпадение"), None)
        assert world_source is not None, "world source fixture is missing"
        world_query = (
            f"/api/world-source-monsters?server=kiss&itemId=1055001"
            f"&sourceLine={world_source['sourceLine']}&groupId={world_source['groupId']}"
            f"&choicePosition={world_source['choicePosition']}&itemPosition={world_source['itemPosition']}"
        )
        world_monsters = assert_status(app.base_url, world_query)
        assert world_monsters["contextMatchKnown"] is False
        assert world_monsters["monsters"], "world source did not expand to level/type candidates"

        hostile = assert_status(app.base_url, "/api/monsters/85?server=kiss")
        assert hostile["monster"]["name"] == "Враждебный дух"
        assert hostile.get("worldRuleCount", 0) > 0, "hostile spirit exposes no world-drop rules"
        hostile_world = assert_status(app.base_url, "/api/monster-world-drops?server=kiss&monsterId=85")
        assert hostile_world["contextMatchKnown"] is False
        soul_beads = []
        golden_chest = []
        for slot in hostile_world["slots"]:
            for choice in slot.get("choices", []):
                for row in choice.get("items", []):
                    if row.get("itemId") == 835221:
                        soul_beads.append(row)
                    if row.get("itemId") == 808100:
                        golden_chest.append(row)
        assert sorted(round(row["baseAttemptChance"], 10) for row in soul_beads) == [0.05, 0.1, 0.85], soul_beads
        assert len(golden_chest) == 1 and abs(golden_chest[0]["baseAttemptChance"] - 0.36) < 1e-12, golden_chest

        kiss = assert_status(app.base_url, "/api/monsters/10042?server=kiss")
        original = assert_status(app.base_url, "/api/monsters/10042?server=original")
        assert kiss["monster"]["id"] == original["monster"]["id"] == 10042

    print("API smoke test: PASS")


if __name__ == "__main__":
    main()
