import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class Version205UpdateHardeningTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.script = (ROOT / "web" / "app.js").read_text(encoding="utf-8")
        cls.styles = (ROOT / "web" / "styles.css").read_text(encoding="utf-8")
        cls.update_go = (ROOT / "update_check.go").read_text(encoding="utf-8")
        cls.server_go = (ROOT / "server.go").read_text(encoding="utf-8")
        cls.community_go = (ROOT / "community.go").read_text(encoding="utf-8")

    def test_update_check_has_independent_sources_and_bounded_networking(self):
        self.assertIn("githubLatestReleaseURL", self.update_go)
        self.assertIn("githubLatestReleaseAPI", self.update_go)
        self.assertIn("http.MethodHead", self.update_go)
        self.assertIn("http.MethodGet", self.update_go)
        self.assertIn("maxUpdateResponseBytes", self.update_go)
        self.assertIn("ResponseHeaderTimeout", self.update_go)
        self.assertIn("TLSHandshakeTimeout", self.update_go)
        self.assertIn("CheckRedirect", self.update_go)
        self.assertIn("http.ErrUseLastResponse", self.update_go)

    def test_update_failures_are_actionable_and_do_not_leak_diagnostics(self):
        self.assertNotIn("Статус неизвестен", self.script)
        for text in (
            "Нет подключения к интернету",
            "Нет доступа к GitHub",
            "GitHub не ответил вовремя",
            "GitHub временно ограничил частоту запросов",
            "GitHub временно недоступен",
            "GitHub вернул некорректный ответ",
        ):
            self.assertIn(text, self.script)
        self.assertIn("diagnostic        string", self.update_go)
        self.assertNotIn("diagnostic        string `json:", self.update_go)
        self.assertIn("result.diagnostic", self.server_go)

    def test_rate_limit_is_respected_without_retry_storm(self):
        self.assertIn("updateRetryCooldown(c.result)", self.update_go)
        self.assertIn("return c.result", self.update_go)
        self.assertIn("retryAfter = 60", self.update_go)
        self.assertIn("automaticUpdateRetries >= 1", self.script)
        self.assertIn("24 * 60 * 60_000", self.script)
        self.assertIn("retryAfterSeconds", self.script)

    def test_last_known_update_status_survives_temporary_failures(self):
        self.assertIn("c.lastGood = fresh", self.update_go)
        self.assertIn("fallback.Stale = true", self.update_go)
        self.assertIn("const hasKnownStatus = Boolean(previous.checked);", self.script)
        self.assertIn("stale: hasKnownStatus", self.script)
        self.assertIn("Последняя успешная проверка", self.script)

    def test_release_links_are_fail_closed(self):
        self.assertIn("function trustedUpdateReleaseURL", self.script)
        self.assertIn("parsed.protocol !== 'https:'", self.script)
        self.assertIn("parsed.hostname !== 'github.com'", self.script)
        self.assertIn("parsed.username || parsed.password", self.script)
        self.assertIn("!trustedPath", self.script)
        self.assertIn('redirect.Port() != "443"', self.update_go)

    def test_community_github_fallback_uses_current_api_contract(self):
        self.assertIn('X-GitHub-Api-Version", "2026-03-10"', self.community_go)
        request_url_start = self.community_go.index("requestURL := target")
        request_create = self.community_go.index(
            "request, err := http.NewRequestWithContext", request_url_start
        )
        request_url_block = self.community_go[request_url_start:request_create]
        self.assertIn("if !githubAPI {", request_url_block)
        self.assertEqual(request_url_block.count('query.Set("refresh"'), 1)

    def test_update_status_has_error_and_stale_visual_states(self):
        self.assertIn('data-status="error"', self.styles)
        self.assertIn('data-status="stale"', self.styles)


if __name__ == "__main__":
    unittest.main()
