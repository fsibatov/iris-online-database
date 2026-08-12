import gzip
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "build_monster_presence_asset", ROOT / "tools/build_monster_presence_asset.py"
)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class MonsterPresenceTests(unittest.TestCase):
    def test_embedded_presence_counts_and_known_differences(self):
        with gzip.open(
            ROOT / "assets/monster_presence.json.gz", "rt", encoding="utf-8"
        ) as handle:
            data = json.load(handle)
        self.assertEqual(data["schemaVersion"], 1)
        original = set(data["servers"]["original"])
        kiss = set(data["servers"]["kiss"])
        self.assertEqual(len(original), 609)
        self.assertEqual(len(kiss), 677)
        self.assertEqual(original - kiss, {11026})
        self.assertEqual(len(kiss - original), 69)
        self.assertIn(85, original)
        self.assertIn(85, kiss)
        self.assertNotIn(253, original)
        self.assertNotIn(253, kiss)

    def test_parser_uses_spawn_rows_and_ignores_commented_rows(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            original = root / "or"
            kiss = root / "kiss"
            original.mkdir()
            kiss.mkdir()
            fixture = """\
1
{
  1 50 100 10000 0 0 1 2
  100 1 2 3 4 0 0 0
}
2
{
  0 0 0 0 0 0 0
  200 1 2 3 4 0 0 0
}
//3
//{
//  1 50 100 10000 0 0 0
//  300 1 2 3 4 0 0 0
//}
"""
            (original / "monsterregen1_0.txt").write_text(fixture, encoding="ascii")

            (kiss / "monsterregen1_0.txt").write_text(
                "4\n{\n 1 50 3s00 10000 0 0 2 5 6\n 400 1 2 3 4 0 0 0\n}\n",
                encoding="ascii",
            )
            self.assertEqual(MODULE.monster_ids(original), [100, 200])
            self.assertEqual(MODULE.monster_ids(kiss), [400])

    def test_gzip_output_is_reproducible(self):
        data = {"schemaVersion": 1, "servers": {"original": [1, 2], "kiss": [1, 3]}}
        with tempfile.TemporaryDirectory() as tmp:
            a = Path(tmp) / "a.gz"
            b = Path(tmp) / "b.gz"
            MODULE.write_gzip_json(a, data)
            MODULE.write_gzip_json(b, data)
            self.assertEqual(a.read_bytes(), b.read_bytes())


if __name__ == "__main__":
    unittest.main()
