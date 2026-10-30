"""Guard the Kiss direct-drop duplicate recorded in the primary source.

Provenance: Library /Iris Resourses.rar, SHA-256
2df78a9b5d4cf659462c8c08a42b2e0e6c5463209079c046576cf5e3aa3c3565;
1/kiss/item_dropn.txt, line 2483, SHA-256
c880b3613ea810c84bdb06f281172a18932d9c9309c191290d5e639f7b292931.
The original source archive is not redistributed in this repository.
"""

import gzip
import json
import unittest
from pathlib import Path

from update_drop_slots import parse_pairs


ROOT = Path(__file__).resolve().parents[1]


class SourceDuplicateDropTests(unittest.TestCase):
    def test_parser_preserves_repeated_group_in_original_order(self):
        columns = "monster\t10042\t0\t0\t0\t0\t6000501\t1000000\t6000501\t500000".split("\t")
        self.assertEqual(
            parse_pairs(columns, 6),
            [{"groupId": 6000501, "chance": 100.0},
             {"groupId": 6000501, "chance": 50.0}],
        )

    def test_source_item_is_resolvable_in_both_repeated_choices(self):
        with gzip.open(ROOT / "assets/game_data.json.gz", "rt", encoding="utf-8") as source:
            data = json.load(source)
        server = data["servers"]["kiss"]
        row = next(slot for slot in server["directSlots"]["10042"] if slot["sourceLine"] == 2483)
        self.assertEqual([choice["groupId"] for choice in row["choices"]], [6000501, 6000501])
        self.assertEqual(server["dropLists"]["6000501"][0]["itemId"], 809022)
        self.assertTrue(any(item["id"] == 809022 for item in data["items"]))
        self.assertTrue(any(monster["id"] == 10042 for monster in data["monsters"]))

    def test_primary_duplicate_item_positions_remain_distinct(self):
        # Iris Resourses.rar: 1/{kiss,or}/item_dropn.txt line 42,
        # item_droplist.txt group 813 contains item 180201 at rows 5 and 6.
        with gzip.open(ROOT / "assets/game_data.json.gz", "rt", encoding="utf-8") as source:
            data = json.load(source)
        for server_name in ("kiss", "original"):
            with self.subTest(server=server_name):
                server = data["servers"][server_name]
                slot = next(slot for slot in server["directSlots"]["8"] if slot["sourceLine"] == 42)
                self.assertEqual(slot["choices"][2]["groupId"], 813)
                rows = server["dropLists"]["813"]
                self.assertEqual([index + 1 for index, row in enumerate(rows)
                                  if row["itemId"] == 180201], [5, 6])
                self.assertEqual(rows[4], rows[5])
        self.assertTrue(any(item["id"] == 180201 for item in data["items"]))
        self.assertTrue(any(monster["id"] == 8 for monster in data["monsters"]))

    def test_primary_source_tiny_chance_is_not_rounded_to_four_decimals(self):
        # Primary RAR, 1/{kiss,or}/item_dropn.txt line 714:
        # monster 135, group 13531, raw group weight 2000 (0.2%).
        # item_droplist.txt line 9843: item 2406, raw weight 625 (0.0625%).
        # Product per main attempt is exactly 0.000125%, not 0.0001%.
        with gzip.open(ROOT / "assets/game_data.json.gz", "rt", encoding="utf-8") as source:
            data = json.load(source)
        for server_name in ("kiss", "original"):
            with self.subTest(server=server_name):
                server = data["servers"][server_name]
                slot = next(slot for slot in server["directSlots"]["135"]
                            if slot["sourceLine"] == 714)
                group = next(choice for choice in slot["choices"]
                             if choice["groupId"] == 13531)
                item = next(row for row in server["dropLists"]["13531"]
                            if row["itemId"] == 2406)
                self.assertEqual(item["quantity"], 1)
                self.assertAlmostEqual(group["chance"], 0.2)
                self.assertAlmostEqual(item["chance"], 0.0625)
                self.assertAlmostEqual(group["chance"] * item["chance"] / 100, 0.000125)

    def test_kiss_projection_preserves_both_source_choices(self):
        with gzip.open(ROOT / "assets/game_data.json.gz", "rt", encoding="utf-8") as source:
            data = json.load(source)
        kiss_slots = data["servers"]["kiss"]["directSlots"]["10042"]
        row = next(slot for slot in kiss_slots if slot["sourceLine"] == 2483)
        self.assertEqual(
            row["choices"],
            [{"groupId": 6000501, "chance": 100.0},
             {"groupId": 6000501, "chance": 50.0}],
        )
        original = data["servers"]["original"]["directSlots"]
        self.assertNotIn(2483, [slot["sourceLine"] for slot in original.get("10042", [])])


if __name__ == "__main__":
    unittest.main()
