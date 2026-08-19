import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class LineEndingsContractTests(unittest.TestCase):
    def test_language_sources_are_checked_out_with_lf_on_windows(self):
        attributes = (ROOT / ".gitattributes").read_text(encoding="utf-8")
        lines = attributes.splitlines()
        self.assertIn("*.go text eol=lf", lines)
        self.assertIn("*.py text eol=lf", lines)

    def test_windows_launchers_use_crlf_checkout(self):
        attributes = (ROOT / ".gitattributes").read_text(encoding="utf-8")
        lines = attributes.splitlines()
        self.assertIn("*.ps1 text eol=crlf", lines)
        self.assertIn("*.bat text eol=crlf", lines)


if __name__ == "__main__":
    unittest.main()
