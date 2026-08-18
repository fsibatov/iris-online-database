import tempfile
import unittest
from pathlib import Path

from drop_table_audit import (
    CHANCE_SCALE,
    DropRestriction,
    DropRule,
    GroupItem,
    WeightedEntry,
    additional_attempts,
    audit,
    effective_interval_weight,
    event_attempts,
    quest_roll_selects,
    reference_item_pick,
    weighted_pick,
    world_rule_applies,
)


class WeightedChoiceTests(unittest.TestCase):
    def test_exact_total_boundaries_are_one_based_and_inclusive(self):
        entries = (
            WeightedEntry(1, 400000),
            WeightedEntry(2, 350000),
            WeightedEntry(3, 250000),
        )
        self.assertEqual(weighted_pick(entries, 1), 1)
        self.assertEqual(weighted_pick(entries, 400000), 1)
        self.assertEqual(weighted_pick(entries, 400001), 2)
        self.assertEqual(weighted_pick(entries, 750000), 2)
        self.assertEqual(weighted_pick(entries, 750001), 3)
        self.assertEqual(weighted_pick(entries, CHANCE_SCALE), 3)

    def test_underflow_and_overflow_match_cumulative_server_choice(self):
        under = (WeightedEntry(1, 400000), WeightedEntry(2, 500000))
        self.assertEqual(weighted_pick(under, 900000), 2)
        self.assertIsNone(weighted_pick(under, 900001))
        self.assertIsNone(weighted_pick(under, CHANCE_SCALE))
        over = (
            WeightedEntry(1, 700000),
            WeightedEntry(2, 500000),
            WeightedEntry(3, 100000),
        )
        self.assertEqual(weighted_pick(over, 700000), 1)
        self.assertEqual(weighted_pick(over, 700001), 2)
        self.assertEqual(weighted_pick(over, CHANCE_SCALE), 2)
        self.assertEqual(effective_interval_weight(over, 0), 700000)
        self.assertEqual(effective_interval_weight(over, 1), 300000)
        self.assertEqual(effective_interval_weight(over, 2), 0)

    def test_roll_range_is_exact(self):
        entries = (WeightedEntry(1, CHANCE_SCALE),)
        for roll in (0, CHANCE_SCALE + 1):
            with self.assertRaises(ValueError):
                weighted_pick(entries, roll)


class AttemptTests(unittest.TestCase):
    def rule(self, a1c=2, a1r=200000, a2c=1, a2r=300000):
        return DropRule(1, 42, a1c, a1r, a2c, a2r, (WeightedEntry(7, CHANCE_SCALE),))

    def test_additional_attempt_boundaries(self):
        rule = self.rule()
        self.assertEqual(additional_attempts(rule, 1), 3)
        self.assertEqual(additional_attempts(rule, 200000), 3)
        self.assertEqual(additional_attempts(rule, 200001), 2)
        self.assertEqual(additional_attempts(rule, 500000), 2)
        self.assertEqual(additional_attempts(rule, 500001), 1)
        self.assertEqual(additional_attempts(rule, CHANCE_SCALE), 1)

    def test_world_attempt_sum_is_clamped(self):
        rule = self.rule(a1c=2, a1r=800000, a2c=4, a2r=800000)
        self.assertEqual(additional_attempts(rule, 800000, world=True), 3)
        self.assertEqual(additional_attempts(rule, 800001, world=True), 5)
        self.assertEqual(additional_attempts(rule, CHANCE_SCALE, world=True), 5)

    def test_event_fallback_attempt_count(self):
        self.assertEqual(event_attempts(0, 0), 0)
        self.assertEqual(event_attempts(250, 49), 3)
        self.assertEqual(event_attempts(250, 50), 2)
        self.assertEqual(event_attempts(250, 99), 2)

    def test_quest_branch_is_independent_and_conditioned(self):
        self.assertTrue(quest_roll_selects(40, 1))
        self.assertTrue(quest_roll_selects(40, 40))
        self.assertFalse(quest_roll_selects(40, 41))
        self.assertFalse(quest_roll_selects(100, 1, conditions_met=False))


class ItemSelectionTests(unittest.TestCase):
    def items(self):
        return (
            GroupItem(10, 7, 1, 100, 400000, 1),
            GroupItem(11, 7, 2, 200, 350000, 2),
            GroupItem(12, 7, 3, 300, 250000, 1),
        )

    def test_item_boundaries_and_quantity(self):
        items = self.items()
        self.assertEqual(reference_item_pick(items, 1).item_id, 100)
        self.assertEqual(reference_item_pick(items, 400000).item_id, 100)
        selected = reference_item_pick(items, 400001)
        self.assertEqual((selected.item_id, selected.quantity), (200, 2))
        self.assertEqual(reference_item_pick(items, CHANCE_SCALE).item_id, 300)

    def test_penalty_changes_integer_cumulative_boundaries(self):
        items = self.items()
        self.assertEqual(reference_item_pick(items, 200000, penalty=0.5).item_id, 100)
        self.assertEqual(reference_item_pick(items, 200001, penalty=0.5).item_id, 200)
        self.assertEqual(reference_item_pick(items, 500001, penalty=0.5).status, "none")

    def test_time_weight_adjusts_whole_cumulative_boundary(self):
        items = self.items()
        restrictions = {200: DropRestriction(1, 200, 1000, 0, 0, 0.5, 2.0)}

        picked = reference_item_pick(
            items, 400001, restrictions=restrictions, period="am"
        )
        self.assertEqual(picked.item_id, 300)

    def test_disabled_restricted_item_is_skipped_but_boundary_is_preserved(self):
        items = self.items()
        restrictions = {100: DropRestriction(1, 100, 1000, 0, 0, 1.0, 1.0)}
        picked = reference_item_pick(
            items, 1, restrictions=restrictions, disabled_restricted_items={100}
        )
        self.assertEqual(picked.item_id, 200)

    def test_duplicate_row_returns_false_instead_of_falling_through(self):
        items = self.items()
        selected_rows = set()
        first = reference_item_pick(items, 1, selected_rows=selected_rows)
        second = reference_item_pick(items, 1, selected_rows=selected_rows)
        self.assertEqual(first.status, "selected")
        self.assertEqual(second.status, "duplicate")
        self.assertEqual(second.item_id, 100)


class WorldRuleTests(unittest.TestCase):
    def test_field_and_instance_are_conditions_not_map_ids(self):
        self.assertTrue(world_rule_applies(1, is_normal_map=True))
        self.assertFalse(world_rule_applies(1, is_normal_map=False))
        self.assertFalse(world_rule_applies(2, is_normal_map=True))
        self.assertTrue(world_rule_applies(2, is_normal_map=False))


class FileAuditTests(unittest.TestCase):
    def test_parser_preserves_attempts_order_quantity_and_sources(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            normal = root / "Item_DropN.txt"
            groups = root / "item_droplist.txt"
            world = root / "Item_DropW.txt"
            limits = root / "item_droplimit.txt"
            penalty = root / "DropIncrease.txt"
            quest = root / "Item_DropQ.txt"
            normal.write_text(
                "monster\t42\t2\t200000\t1\t300000\t100\t400000\t101\t600000\n",
                encoding="utf-8",
            )
            groups.write_text(
                "100\t200\t400000\t2\n100\t201\t600000\t1\n101\t202\t1000000\t1\n",
                encoding="utf-8",
            )
            world.write_text(
                "world\t1\t99\t2\t7\t3\t100000\t4\t200000\t101\t1000000\n",
                encoding="utf-8",
            )
            limits.write_text("200\t1000\t0.1\t0.2\t0.5\t1.5\n", encoding="utf-8")
            penalty.write_text(
                "0\n{\n-10\t0.8\n0\t1\n}\n1\n{\n-10\t0.7\n0\t1\n}\n", encoding="utf-8"
            )
            quest.write_text("31\t42\t870003\t80\n", encoding="utf-8")
            result = audit(normal, groups, world, limits, penalty, quest)
            self.assertEqual(result.normal_rows_with_additional_attempts, 1)
            self.assertEqual(result.world_rows_with_additional_attempts, 1)
            self.assertEqual(result.quantity_over_one_rows, 1)
            self.assertEqual(result.restricted_items, 1)
            self.assertEqual(result.penalty_rows, 4)
            self.assertEqual(result.quest_rows, 1)
            self.assertEqual(result.quest_monsters, 1)
            self.assertEqual(result.world_server_type_values, {2: 1})
            self.assertFalse(result.missing_group_references)


if __name__ == "__main__":
    unittest.main()
