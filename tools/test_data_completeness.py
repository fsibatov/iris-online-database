import importlib.util
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "data_presentation_audit", ROOT / "tools/data_presentation_audit.py"
)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class DataCompletenessTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.result = MODULE.audit()

    def test_all_embedded_fields_are_classified_and_retained_by_go(self):
        checks = self.result["checks"]
        for key in (
            "item_unclassified",
            "monster_unclassified",
            "server_unclassified",
            "item_missing_go_fields",
            "monster_missing_go_fields",
            "server_missing_go_fields",
            "effect_spec_missing_go_fields",
        ):
            self.assertEqual(checks[key], [], key)

    def test_all_set_rows_and_thresholds_are_present(self):
        self.assertEqual(self.result["setsWithEffectDefinitions"], 458)
        self.assertEqual(self.result["setEffectRows"], 972)
        self.assertEqual(self.result["setThresholds"], [2, 3, 4, 5])
        self.assertGreater(self.result["setThresholdRowCounts"][5], 0)
        self.assertGreater(self.result["fivePieceActiveEffects"], 0)
        self.assertEqual(self.result["maxDistinctThresholdsPerSet"], 4)
        self.assertGreaterEqual(self.result["maxRowsAtSameThreshold"], 2)
        self.assertGreaterEqual(self.result["maxEffectLinesAtSameThreshold"], 2)
        self.assertEqual(self.result["exactDuplicateSetRows"], 16)
        self.assertEqual(self.result["setsWithExactDuplicateRows"], 4)

    def test_set_enums_and_card_slots_are_not_silently_unknown(self):
        checks = self.result["checks"]
        self.assertEqual(checks["unknown_set_effect_types"], {})
        self.assertEqual(checks["unknown_active_states"], {})
        self.assertEqual(checks["unknown_card_slot_types"], [])

    def test_item_ability_projection_restores_missing_rows_without_overwriting_conflicts(
        self,
    ):
        self.assertEqual(self.result["itemAbilitySupplementItems"], 13927)
        self.assertEqual(
            self.result["itemAbilitySupplementFieldCounts"]["options"], 783
        )
        self.assertEqual(
            self.result["itemAbilitySupplementFieldCounts"]["physicalDefense"], 95
        )
        self.assertEqual(
            self.result["itemAbilitySupplementFieldCounts"]["magicDefense"], 95
        )
        self.assertEqual(self.result["preservedRawAbilityConflicts"], 38)
        self.assertEqual(self.result["restoredAbilityDescriptions"], 3086)
        self.assertEqual(self.result["restoredItemLimitUsageRules"], 632)
        self.assertEqual(self.result["restoredProfessionRules"], 1234)
        self.assertEqual(self.result["restoredGuildRules"], 4)
        checks = self.result["checks"]
        self.assertEqual(checks["ability_supplement_unclassified_fields"], [])
        self.assertEqual(checks["unknown_make_skill_codes"], [])
        self.assertEqual(checks["unknown_use_map_codes"], [])
        self.assertEqual(checks["unknown_guild_use_codes"], [])

    def test_unknown_item_effect_enum_is_reported_not_dropped(self):
        self.assertEqual(self.result["unknownItemEffectTypes"], {320: 2})
        script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        go = (ROOT / "main.go").read_text(encoding="utf-8")
        self.assertIn("Неизвестный эффект (код ${option.type})", script)
        self.assertIn("Неизвестный эффект (код %d)", go)

    def test_explicit_zero_item_options_are_preserved(self):
        self.assertGreater(self.result["explicitZeroItemOptions"], 0)
        script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        self.assertIn("{ allowZero: true }", script)

    def test_no_set_effect_limit_or_monster_id_preview_regression(self):
        checks = self.result["checks"]
        self.assertFalse(checks["frontend_has_set_slice_limit"])
        self.assertFalse(checks["frontend_uses_generic_properties_array"])
        self.assertFalse(checks["monster_id_in_suggestion_preview"])
        script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        self.assertNotIn("group.seen.has(row)", script)
        self.assertIn("group.rows.push(row)", script)

    def test_restored_descriptions_and_restrictions_have_explicit_presentation_paths(
        self,
    ):
        script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        self.assertIn("item.abilityDescription", script)
        self.assertIn("bonusTexts", script)
        self.assertIn("MAKE_SKILL_NAMES", script)
        self.assertIn("USE_MAP_NAMES", script)
        self.assertIn("GUILD_USE_NAMES", script)
        self.assertIn("item.degradationIndex", script)
        self.assertIn("item.enhancedIndex", script)
        self.assertIn("item.printableFlag", script)

    def test_chest_projection_is_complete_and_fail_closed(self):
        self.assertEqual(
            self.result["chestProfilesByServer"], {"kiss": 399, "original": 399}
        )
        self.assertEqual(
            self.result["chestItemRowsByServer"], {"kiss": 4944, "original": 4944}
        )
        self.assertEqual(
            self.result["chestContentsSha256"],
            "2b77350006cfb7f992c1707b9d0c703ebb82206dac1810084aa105d0878fe98b",
        )
        checks = self.result["checks"]
        self.assertEqual(checks["chest_source_ids_missing_from_game_data"], [])
        self.assertEqual(checks["invalid_chest_profiles"], [])
        self.assertEqual(
            checks["chest_probability_unknown_profiles"],
            ["kiss:873079", "original:873079"],
        )
        self.assertFalse(checks["chest_supplement_not_merged"])
        self.assertFalse(checks["chest_missing_item_fallback_absent"])

        self.assertEqual(self.result["chestOutputIDsMissingFromGameData"], 24)

    def test_item_rank_is_not_mislabeled_as_level(self):
        script = (ROOT / "web/app.js").read_text(encoding="utf-8")
        self.assertIn("`Ранг ${item.level}`", script)
        self.assertIn("'По рангу'", script)
        self.assertIn("'Ранг от'", script)


if __name__ == "__main__":
    unittest.main()
