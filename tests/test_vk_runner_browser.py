import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / ".github" / "workflows" / "update-vk-news.yml"


class VKRunnerBrowserTests(unittest.TestCase):
    def test_scheduled_vk_workflow_uses_runner_browser(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("runs-on: windows-2025", workflow)
        self.assertIn('"Google\\Chrome\\Application\\chrome.exe"', workflow)
        self.assertIn("Get-Command chrome.exe", workflow)
        self.assertIn("PLAYWRIGHT_CHROMIUM_EXECUTABLE", workflow)
        self.assertNotIn("playwright install --with-deps chromium", workflow)
        self.assertNotIn("python -m playwright install", workflow)
        self.assertNotIn("ubuntu-", workflow)
        self.assertNotIn("command -v", workflow)


if __name__ == "__main__":
    unittest.main()
