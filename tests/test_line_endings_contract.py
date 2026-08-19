import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class LineEndingsContractTests(unittest.TestCase):
    def test_go_sources_are_checked_out_with_lf_on_all_platforms(self):
        attributes = (ROOT / ".gitattributes").read_text(encoding="utf-8")
        self.assertIn("*.go text eol=lf", attributes.splitlines())


if __name__ == "__main__":
    unittest.main()
