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

    def test_primary_same_item_across_distinct_monster_slots(self):
        # Iris Resourses.rar, monster 10026, group 1002607, item 868026.
        # Kiss item_dropn.txt lines 1327/1328; Original lines 1334/1335.
        # Each slot is an independent source, not the active selection of both.
        with gzip.open(ROOT / "assets/game_data.json.gz", "rt", encoding="utf-8") as source:
            data = json.load(source)
        for server_name, source_lines in (("kiss", (1327, 1328)),
                                          ("original", (1334, 1335))):
            with self.subTest(server=server_name):
                server = data["servers"][server_name]
                slots = server["directSlots"]["10026"]
                selected = [slot for slot in slots if slot["sourceLine"] in source_lines]
                self.assertEqual([slot["sourceLine"] for slot in selected], list(source_lines))
                for slot in selected:
                    self.assertEqual(slot["choices"][0]["groupId"], 1002607)
                self.assertEqual(server["dropLists"]["1002607"],
                                 [{"itemId": 868026, "chance": 100.0, "quantity": 1}])
        self.assertTrue(any(item["id"] == 868026 for item in data["items"]))

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

    def test_primary_tiny_drop_chance_preserves_ten_decimal_places(self):
        # Iris Resourses.rar: both 1/{kiss,or}/item_dropn.txt line 568
        # monster 104, group 10431 raw weight 42 (0.0042%).
        # item_droplist.txt line 7421, item 131651 raw weight 833
        # (0.0833%), quantity 1. Product: 0.0000034986% per attempt.
        with gzip.open(ROOT / "assets/game_data.json.gz", "rt", encoding="utf-8") as source:
            data = json.load(source)
        for server_name in ("kiss", "original"):
            with self.subTest(server=server_name):
                server = data["servers"][server_name]
                slot = next(slot for slot in server["directSlots"]["104"]
                            if slot["sourceLine"] == 568)
                choice = next(choice for choice in slot["choices"]
                              if choice["groupId"] == 10431)
                row = next(row for row in server["dropLists"]["10431"]
                           if row["itemId"] == 131651)
                self.assertEqual(row["quantity"], 1)
                self.assertAlmostEqual(choice["chance"], 0.0042)
                self.assertAlmostEqual(row["chance"], 0.0833)
                self.assertAlmostEqual(choice["chance"] * row["chance"] / 100, 0.0000034986)

    def test_primary_same_item_distinct_quantities_in_both_servers(self):
        # Iris Resourses.rar: monster 148, direct line 775, group 14801,
        # item 835243 at source list lines 10894/10895 (quantity 1 / 5).
        with gzip.open(ROOT / "assets/game_data.json.gz", "rt", encoding="utf-8") as source:
            data = json.load(source)
        for server_name in ("kiss", "original"):
            with self.subTest(server=server_name):
                server = data["servers"][server_name]
                direct = next(slot for slot in server["directSlots"]["148"]
                              if slot["sourceLine"] == 775)
                self.assertEqual(direct["choices"][0]["groupId"], 14801)
                variants = [(index + 1, row["itemId"], row["quantity"], row["chance"])
                            for index, row in enumerate(server["dropLists"]["14801"])
                            if row["itemId"] == 835243]
                self.assertEqual(variants, [(2, 835243, 1, 1.0), (3, 835243, 5, 0.1)])

    def test_primary_full_weight_precedes_unreachable_group_in_both_servers(self):
        # Primary Iris Resourses.rar: 1/{kiss,or}/item_dropn.txt line 1287,
        # monster 15019 (Гарри), group 1501902 weight 1,000,000 then
        # group 1501912 weight 20,000. The second group is present but has
        # a known 0% effective selection probability on the cumulative roll.
        # 1/kiss/item_droplist.txt line 22799, Original line 22797:
        # item 81108 remains listed with raw weight 71,428 and quantity 1.
        with gzip.open(ROOT / "assets/game_data.json.gz", "rt", encoding="utf-8") as source:
            data = json.load(source)
        for server_name in ("kiss", "original"):
            with self.subTest(server=server_name):
                server = data["servers"][server_name]
                slot = next(row for row in server["directSlots"]["15019"]
                            if row["sourceLine"] == 1287)
                self.assertEqual([(choice["groupId"], choice["chance"])
                                  for choice in slot["choices"]],
                                 [(1501902, 100.0), (1501912, 2.0)])
                self.assertEqual(max(0, min(100, 102.0) - min(100, 100.0)), 0)
                self.assertEqual(server["dropLists"]["1501912"][0],
                                 {"itemId": 81108, "chance": 7.1428, "quantity": 1})

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
