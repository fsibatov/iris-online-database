import gzip
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ASSET = ROOT / "assets/chest_contents.json.gz"
BUILDER_PATH = ROOT / "tools/build_chest_contents_asset.py"
SPEC = importlib.util.spec_from_file_location(
    "build_chest_contents_asset", BUILDER_PATH
)
BUILDER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BUILDER)


class ChestContentTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with gzip.open(ASSET, "rt", encoding="utf-8") as handle:
            cls.data = json.load(handle)

    def test_embedded_projection_counts_and_fixture(self):
        self.assertEqual(self.data["schemaVersion"], 1)
        for server in ("kiss", "original"):
            profiles = self.data["servers"][server]["profiles"]
            self.assertEqual(len(profiles), 399)
            self.assertEqual(
                sum(len(profile["rows"]) for profile in profiles.values()), 4944
            )
            profile = profiles["808094"]
            self.assertEqual(profile["drawCount"], 1)
            row = next(row for row in profile["rows"] if row["itemId"] == 101402)
            self.assertEqual(
                row,
                {
                    "itemId": 101402,
                    "quantity": 1,
                    "enhanced": 0,
                    "threshold": 1000000,
                    "position": 31,
                },
            )

    def test_builder_preserves_source_order_and_filters_non_containers(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            game = root / "game.json.gz"
            abilities = root / "abilities.json.gz"
            with gzip.open(game, "wt", encoding="utf-8") as handle:
                json.dump(
                    {
                        "items": [
                            {
                                "id": 10,
                                "middleCategoryId": 603,
                                "subcategory": "Награда за квест",
                            },
                            {
                                "id": 20,
                                "middleCategoryId": 407,
                                "subcategory": "Сундук",
                            },
                        ]
                    },
                    handle,
                    ensure_ascii=False,
                )
            with gzip.open(abilities, "wt", encoding="utf-8") as handle:
                json.dump(
                    {
                        "items": {
                            "10": {"kindOf": 3, "eventType": 3, "changeIndex": 10},
                            "20": {"kindOf": 1, "eventType": 0, "changeIndex": 0},
                        }
                    },
                    handle,
                    ensure_ascii=False,
                )
            table = root / "item_change.txt"
            table.write_text(
                "index\t10\nrate\t2\nitem\t101\t102\ncount\t3\t1\nenhanced\t4\t0\nchangerate\t500000\t1000000\n"
                "index\t20\nrate\t1\nitem\t201\ncount\t1\nenhanced\t0\nchangerate\t1000000\n",
                encoding="utf-8",
            )
            built = BUILDER.build(game, abilities, table, table)
            profiles = built["servers"]["kiss"]["profiles"]
            self.assertEqual(list(profiles), ["10"])
            self.assertEqual(profiles["10"]["drawCount"], 2)
            self.assertEqual(
                [row["itemId"] for row in profiles["10"]["rows"]], [101, 102]
            )
            self.assertEqual(profiles["10"]["rows"][0]["quantity"], 3)
            self.assertEqual(profiles["10"]["rows"][0]["enhanced"], 4)
            self.assertEqual(profiles["10"]["rows"][1]["position"], 2)

    def test_non_chest_catalog_category_container_is_retained(self):
        for server in ("kiss", "original"):
            profiles = self.data["servers"][server]["profiles"]
            self.assertIn("873063", profiles)
            self.assertGreater(len(profiles["873063"]["rows"]), 0)

    def test_runtime_and_embed_reference_chest_asset(self):
        go = (ROOT / "main.go").read_text(encoding="utf-8")
        server = (ROOT / "server.go").read_text(encoding="utf-8")
        self.assertIn("mergeChestContentSupplement()", go)
        self.assertIn("assets/chest_contents.json.gz", go + server)
        self.assertIn("chestTierItemChance", go)
        self.assertIn("chestByItem", go)


if __name__ == "__main__":
    unittest.main()
