import copy
import unittest

from raw_projection_audit import option_conflict_mismatches
from verify_executables import release_marker_matches


class AuditRegressionTests(unittest.TestCase):
    def setUp(self):
        self.conflict = {
            "itemId": 151201001,
            "field": "options",
            "embedded": [{"type": 120, "value": 52}],
            "source": [{"type": 107, "value": 67}],
        }

    def test_exact_conflicts_are_accepted_regardless_of_key_order(self):
        reordered = dict(reversed(list(self.conflict.items())))
        self.assertEqual(option_conflict_mismatches([reordered], [self.conflict]), 0)

    def test_changed_value_is_rejected_at_the_same_conflict_count(self):
        changed = copy.deepcopy(self.conflict)
        changed["embedded"][0]["value"] = 9999
        self.assertGreater(option_conflict_mismatches([changed], [self.conflict]), 0)

    def test_conflict_id_source_and_field_cannot_be_replaced(self):
        for key, value in [("itemId", 42), ("source", []), ("field", "price")]:
            with self.subTest(key=key):
                changed = {**self.conflict, key: value}
                self.assertGreater(
                    option_conflict_mismatches([changed], [self.conflict]), 0
                )

    def test_duplicate_missing_and_malformed_exceptions_are_rejected(self):
        for actual, expected in [
            ([self.conflict], [self.conflict, self.conflict]),
            ([], [self.conflict]),
            ([self.conflict], []),
            ([self.conflict], [{**self.conflict, "itemId": True}]),
        ]:
            with self.subTest(actual=actual, expected=expected):
                self.assertGreater(option_conflict_mismatches(actual, expected), 0)

    def test_release_marker_requires_exact_version_and_commit(self):
        commit = "a" * 40
        marker = f"IrisOnlineRelease/2.0.6/{commit}".encode()
        self.assertTrue(
            release_marker_matches(b"prefix\0" + marker + b"\0suffix", "2.0.6", commit)
        )
        for version, expected in [
            ("2.0.5", commit),
            ("2.0.6", "b" * 40),
            ("2.0.6", "a" * 7),
        ]:
            with self.subTest(version=version, expected=expected):
                self.assertFalse(release_marker_matches(marker, version, expected))
        foreign = b"\0IrisOnlineRelease/2.0.6/" + b"b" * 40
        self.assertFalse(release_marker_matches(marker + foreign, "2.0.6", commit))


if __name__ == "__main__":
    unittest.main()
