import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / ".github" / "workflows" / "update-vk-news.yml"
UPDATER = ROOT / "tools" / "update_vk_news.py"


class VKWorkflowTransientPolicyTests(unittest.TestCase):
    def test_workflow_live_smoke_trigger_is_non_recursive(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("push:", workflow)
        self.assertIn("branches: [main]", workflow)
        self.assertIn("- '.github/workflows/update-vk-news.yml'", workflow)
        self.assertIn("- 'tools/update_vk_news.py'", workflow)
        self.assertNotIn("- 'data/latest-vk.json'", workflow)
        self.assertIn("group: update-vk-news-v2", workflow)

    def test_scheduler_uses_one_scrape_path_per_attempt(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("for ($Attempt = 1; $Attempt -le 3; $Attempt++)", workflow)
        self.assertIn('"vk-candidate-$Attempt.json"', workflow)
        self.assertEqual(workflow.count("tools\\update_vk_news.py --output"), 1)
        self.assertNotIn("vk-stale-candidate-", workflow)
        self.assertNotIn("STALE_POST)", workflow)

    def test_scheduler_retries_only_known_transient_vk_failures(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("if ($Result.TimedOut)", workflow)
        self.assertIn('$Category = "TIMEOUT"', workflow)
        for category in ("EMPTY_PREVIEW", "VK_UNAVAILABLE"):
            self.assertIn(category, workflow)
        self.assertIn("BROWSER_*", workflow)
        self.assertIn("VK transient failure", workflow)

    def test_scheduler_parses_ascii_category_from_stderr(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("[string]$Result.Stderr", workflow)
        self.assertIn("[A-Z0-9_:.-]*", workflow)
        self.assertNotIn('"Ошибка обновления VK \\[([^\\]]*)\\]"', workflow)

    def test_lower_id_requires_three_consistent_fresh_scrapes_before_promotion(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("$StaleConfirmations = 0", workflow)
        self.assertIn("$CandidateAction = $Matches[2]", workflow)
        self.assertIn('"stale" {', workflow)
        self.assertIn("$StaleConfirmations++", workflow)
        self.assertIn("if ($StaleConfirmations -eq 3)", workflow)
        self.assertIn(
            'Copy-Item -LiteralPath $CandidateFile -Destination "data\\latest-vk.json" -Force',
            workflow,
        )
        self.assertIn("after 3 independent confirmations", workflow)
        self.assertIn("confirmation counter reset", workflow)

    def test_equal_or_newer_candidate_does_not_need_stale_quorum(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn('action = "same"', workflow)
        self.assertIn('action = "promote"', workflow)
        self.assertIn('"same" {', workflow)
        self.assertIn('"promote" {', workflow)
        self.assertIn("VK: без изменений", workflow)
        self.assertIn("VK: подтверждена актуальная запись", workflow)

    def test_candidate_and_current_json_are_validated_before_comparison(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("json.loads", workflow)
        self.assertIn('payload.get("post_id")', workflow)
        self.assertIn('payload.get("post_url")', workflow)
        self.assertIn('payload.get("text")', workflow)
        self.assertIn("https://vk.ru/wall-59626511_{post_id}", workflow)
        self.assertIn("VK candidate/current JSON validation failed", workflow)

    def test_exhausted_transient_or_unconfirmed_lower_id_is_warning(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("::warning::VK is temporarily unavailable", workflow)
        self.assertIn("Lower-ID VK candidate was not confirmed 3 times", workflow)
        self.assertIn("Last-known-good data is preserved", workflow)

    def test_unknown_or_internal_failure_remains_fail_closed(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn("VK updater failed with non-transient category", workflow)
        self.assertIn("exit $Result.ExitCode", workflow)
        self.assertIn("VK updater ended in an unexpected state", workflow)
        self.assertNotIn("continue-on-error", workflow)

    def test_push_rebases_and_retries_against_current_main(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")

        self.assertIn(
            "for ($PushAttempt = 1; $PushAttempt -le 3; $PushAttempt++)", workflow
        )
        self.assertIn("git fetch origin main", workflow)
        self.assertIn("git rebase origin/main", workflow)
        self.assertIn("git push origin HEAD:main", workflow)
        self.assertIn(
            "Failed to push VK update after 3 synchronized attempts", workflow
        )

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
