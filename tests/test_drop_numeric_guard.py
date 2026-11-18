import tempfile
import unittest
from pathlib import Path

from update_drop_slots import parse_direct_slots


class DirectDropNumericGuardTests(unittest.TestCase):
    def assert_invalid_pair(self, group_text, chance_text):
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "synthetic_dropn.txt"
            path.write_text(
                f"header\nmonster\t123\t0\t0\t0\t0\t{group_text}\t{chance_text}\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(
                ValueError, r"invalid direct-drop numeric pair at line 2"
            ):
                parse_direct_slots(path)

    def test_noninteger_group_id_fails_closed(self):
        self.assert_invalid_pair("not-an-integer", "500000")

    def test_noninteger_weight_fails_closed(self):
        self.assert_invalid_pair("12345", "not-an-integer")
