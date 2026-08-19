import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class LineEndingsContractTests(unittest.TestCase):
    def test_language_and_go_module_sources_use_lf_on_windows(self):
        attributes = (ROOT / ".gitattributes").read_text(encoding="utf-8")
        lines = attributes.splitlines()
        for rule in (
            "*.go text eol=lf",
            "go.mod text eol=lf",
            "go.sum text eol=lf",
            ".go-version text eol=lf",
            "*.py text eol=lf",
            "*.yml text eol=lf",
            "*.yaml text eol=lf",
        ):
            self.assertIn(rule, lines)

    def test_windows_launchers_use_crlf_checkout(self):
        attributes = (ROOT / ".gitattributes").read_text(encoding="utf-8")
        lines = attributes.splitlines()
        self.assertIn("*.ps1 text eol=crlf", lines)
        self.assertIn("*.bat text eol=crlf", lines)


if __name__ == "__main__":
    unittest.main()
