import gzip
import hashlib
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ASSET = ROOT / "assets/quest_reward_sources.json.gz"
BUILDER_PATH = ROOT / "tools/build_quest_reward_sources_asset.py"
SPEC = importlib.util.spec_from_file_location(
    "build_quest_reward_sources_asset", BUILDER_PATH
)
BUILDER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BUILDER)


class QuestRewardSourceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with gzip.open(ASSET, "rt", encoding="utf-8") as handle:
            cls.data = json.load(handle)
        with gzip.open(
            ROOT / "assets/item_abilities.json.gz", "rt", encoding="utf-8"
        ) as handle:
            cls.abilities = json.load(handle)
        with gzip.open(
            ROOT / "assets/item_recipes.json.gz", "rt", encoding="utf-8"
        ) as handle:
            cls.recipes = json.load(handle)

    def test_embedded_projection_counts_and_confirmed_fixtures(self):
        self.assertEqual(
            hashlib.sha256(ASSET.read_bytes()).hexdigest(),
            "705b691a49c61e7dfe8f35ccdb04c5c03ef43f7a42fc1025ff70466ffd8e74cd",
        )
        self.assertEqual(self.data["schemaVersion"], 1)
        rewards = self.data["rewards"]
        self.assertEqual(len(rewards), 81)
        self.assertEqual(len({row["itemId"] for row in rewards}), 75)
        self.assertEqual(len({row["questId"] for row in rewards}), 80)
        self.assertEqual({row["rewardType"] for row in rewards}, {"default"})

        student = [row for row in rewards if row["itemId"] == 807001]
        self.assertEqual([row["questId"] for row in student], [20, 21, 22])
        self.assertTrue(all(row["quest"] == "Первый подвиг" for row in student))

        spider = [row for row in rewards if row["itemId"] == 891002]
        self.assertEqual(
            spider,
            [
                {
                    "itemId": 891002,
                    "questId": 78,
                    "questTitleIndex": 78,
                    "quest": "На что годятся пауки - 1",
                    "rewardType": "default",
                    "quantity": 1,
                }
            ],
        )
        self.assertNotIn(4033, {row["questId"] for row in rewards})

    def test_projection_contains_only_title_items_or_recipes(self):
        title_items = {
            int(item_id)
            for item_id, patch in self.abilities["items"].items()
            if int(patch.get("titleIndex", 0) or 0) > 0
        }
        recipe_items = {int(item_id) for item_id in self.recipes["recipes"]}
        rewards = self.data["rewards"]
        self.assertTrue(
            all(row["itemId"] in title_items | recipe_items for row in rewards)
        )
        title_rows = [row for row in rewards if row["itemId"] in title_items]
        recipe_rows = [row for row in rewards if row["itemId"] in recipe_items]
        title_indexes = {
            int(self.abilities["items"][str(row["itemId"])]["titleIndex"])
            for row in title_rows
        }
        self.assertEqual(len(title_rows), 62)
        self.assertEqual(len(title_indexes), 56)
        self.assertEqual(len(recipe_rows), 19)
        self.assertEqual(len({row["itemId"] for row in recipe_rows}), 19)

    def test_builder_preserves_default_and_select_semantics(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            quest_root = root / "quests"
            quest_root.mkdir()
            (quest_root / "quest_main.txt").write_text(
                "quest 10\n{\n\ttitle 100\n\tdefault\n\t{\n\t\titem 1000 1\n\t}\n}\n"
                "quest 20\n{\n\ttitle 200\n\tselect\n\t{\n\t\titem 2000 2\n\t}\n}\n",
                encoding="cp1251",
            )
            for name in ("quest_extra.txt", "quest_scroll.txt", "quest_ivent.txt"):
                (quest_root / name).write_text("", encoding="cp1251")

            quest_names = root / "quest_name.txt"
            quest_names.write_text(
                '100\t"Первое задание"\n200\t"Второе задание"\n',
                encoding="utf-16",
            )
            abilities = root / "abilities.json.gz"
            recipes = root / "recipes.json.gz"
            with gzip.open(abilities, "wt", encoding="utf-8") as handle:
                json.dump(
                    {"items": {"1000": {"titleIndex": 7}, "2000": {}}},
                    handle,
                )
            with gzip.open(recipes, "wt", encoding="utf-8") as handle:
                json.dump({"recipes": {"2000": []}}, handle)

            built = BUILDER.build(
                quest_root,
                quest_names,
                abilities,
                recipes,
            )
            self.assertEqual(
                built["rewards"],
                [
                    {
                        "itemId": 1000,
                        "questId": 10,
                        "questTitleIndex": 100,
                        "quest": "Первое задание",
                        "rewardType": "default",
                        "quantity": 1,
                    },
                    {
                        "itemId": 2000,
                        "questId": 20,
                        "questTitleIndex": 200,
                        "quest": "Второе задание",
                        "rewardType": "select",
                        "quantity": 2,
                    },
                ],
            )

    def test_backend_frontend_and_embed_reference_projection(self):
        go = (ROOT / "main.go").read_text(encoding="utf-8")
        server = (ROOT / "server.go").read_text(encoding="utf-8")
        script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        self.assertIn("mergeQuestRewardSupplement()", go)
        self.assertIn("assets/quest_reward_sources.json.gz", server)
        self.assertIn('Source:       "Награда за задание"', go)
        self.assertIn("'Награда за задание'", script)
        self.assertIn("ID задания", script)
        self.assertIn("Доступность самого задания", script)


if __name__ == "__main__":
    unittest.main()
