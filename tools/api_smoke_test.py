#!/usr/bin/env python3
"""Reproducible API smoke test for Iris Online."""
from __future__ import annotations

import argparse
from smoke_common import RunningApp, json_request, require_binary


def assert_status(base: str, path: str, expected: int = 200, **kwargs):
    status, payload = json_request(base, path, **kwargs)
    if status != expected:
        raise AssertionError(f"{path}: status {status}, expected {expected}; payload={payload!r}")
    return payload


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    binary = require_binary(args.binary)

    with RunningApp(binary, ["-no-browser"]) as app:
        app.wait_ready()
        health = assert_status(app.base_url, "/api/health")
        assert health["application"] == "iris-online-database"
        assert health["version"] == "1.0"

        search = assert_status(app.base_url, "/api/search?q=%D0%B3%D0%BD%D0%B5%D0%B2%20%D0%BF%D1%80%D0%B5%D0%B4%D0%BA%D0%BE%D0%B2")
        assert search["items"] or search["monsters"], "word-form search returned nothing"

        by_id = assert_status(app.base_url, "/api/items?q=80592&page=1&pageSize=24")
        assert any(row["id"] == 80592 for row in by_id["items"])

        empty = assert_status(app.base_url, "/api/items?q=__definitely_missing__&page=1&pageSize=24")
        assert empty["total"] == 0
        assert_status(app.base_url, "/api/items/999999999", expected=404)

        filtered = assert_status(app.base_url, "/api/items?category=%D0%9E%D1%80%D1%83%D0%B6%D0%B8%D0%B5%2F%D1%89%D0%B8%D1%82&page=1&pageSize=24&sort=level")
        assert filtered["page"] == 1 and len(filtered["items"]) <= 24
        paged = assert_status(app.base_url, "/api/monsters?page=2&pageSize=24&sort=level")
        assert paged["page"] == 2 and len(paged["monsters"]) <= 24
        bounded = assert_status(app.base_url, "/api/items?page=-50&pageSize=999999")
        assert bounded["page"] == 1 and bounded["pageSize"] == 48 and len(bounded["items"]) <= 48
        assert_status(app.base_url, "/api/items?sort=unknown-enum", expected=400)
        assert_status(app.base_url, "/api/items/not-a-number", expected=400)
        assert_status(app.base_url, "/api/monsters/not-a-number", expected=400)
        assert_status(app.base_url, "/api/items", method="POST", expected=405)

        keys: list[str] = []
        page = 1
        while len(keys) < 620:
            data = assert_status(app.base_url, f"/api/items?page={page}&pageSize=48&sort=name")
            keys.extend(f"item:{row['id']}" for row in data["items"])
            if page >= data["pages"]:
                break
            page += 1
        assert len(keys) >= 620, f"only {len(keys)} item IDs available"
        favorites = assert_status(
            app.base_url,
            "/api/favorites",
            method="POST",
            payload={"keys": keys[:620], "server": "kiss", "page": 13, "pageSize": 50},
        )
        assert favorites["total"] == 620
        assert favorites["pages"] == 13
        assert len(favorites["rows"]) == 20

        kiss = assert_status(app.base_url, "/api/monsters/10042?server=kiss")
        original = assert_status(app.base_url, "/api/monsters/10042?server=original")
        assert kiss["monster"]["id"] == original["monster"]["id"] == 10042

    print("API smoke test: PASS")


if __name__ == "__main__":
    main()
