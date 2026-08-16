import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / ".github" / "workflows" / "update-vk-news.yml"
UPDATER = ROOT / "tools" / "update_vk_news.py"


class VKWorkflowTransientPolicyTests(unittest.TestCase):
    def test_scheduler_retries_only_known_transient_vk_failures(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("for attempt in 1 2 3", workflow)
        self.assertIn(
            'output="$(timeout 3m python -B tools/update_vk_news.py', workflow
        )
        self.assertIn('if [ "$exit_code" -eq 124 ]', workflow)
        self.assertIn('category="TIMEOUT"', workflow)

        for category in ("EMPTY_PREVIEW", "VK_UNAVAILABLE", "STALE_POST"):
            self.assertIn(category, workflow)
        self.assertIn("BROWSER_*", workflow)
        self.assertIn("VK transient failure", workflow)

    def test_exhausted_transient_failure_is_warning_not_failed_run(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("::warning::VK is temporarily unavailable", workflow)
        self.assertIn("Last-known-good data is preserved", workflow)
        warning = workflow.index("::warning::VK is temporarily unavailable")
        success_exit = workflow.index("exit 0", warning)
        self.assertGreater(success_exit, warning)

    def test_unknown_or_internal_failure_remains_fail_closed(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("VK updater failed with non-transient category", workflow)
        self.assertIn('exit "$exit_code"', workflow)
        self.assertIn("VK updater ended in an unexpected state", workflow)
        self.assertNotIn("continue-on-error", workflow)

    def test_updater_keeps_safe_failure_categories_and_strict_exit(self):
        updater = UPDATER.read_text(encoding="utf-8")

        for category in (
            "EMPTY_PREVIEW",
            "STALE_POST",
            "INVALID_PAYLOAD",
            "MISSING_DEPENDENCY",
            "VK_UNAVAILABLE",
            "LOCAL_IO_FAILURE",
            "UPDATE_FAILED",
        ):
            self.assertIn(category, updater)

        self.assertIn("return 1", updater)
        self.assertIn("last-known-good data was preserved", updater)


if __name__ == "__main__":
    unittest.main()
