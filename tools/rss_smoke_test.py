"""Short RSS smoke test. This is not proof that memory leaks are absent."""

from __future__ import annotations

import argparse
import gc
import time

import psutil
from smoke_common import RunningApp, json_request, require_binary


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    parser.add_argument("--iterations", type=int, default=1200)
    args = parser.parse_args()
    binary = require_binary(args.binary)

    with RunningApp(binary, ["-no-browser"]) as app:
        app.wait_ready()
        process = psutil.Process(app.process.pid)
        favorite_keys = [
            "item:80567",
            "item:80568",
            "item:80569",
            "monster:10042",
            "monster:141",
        ]
        for index in range(80):
            json_request(
                app.base_url, f"/api/items?page={(index % 10) + 1}&pageSize=48"
            )
        time.sleep(0.8)
        before = process.memory_info().rss
        for index in range(args.iterations):
            server = "original" if index % 2 else "kiss"
            kind = index % 8
            if kind == 0:
                status, _ = json_request(
                    app.base_url,
                    f"/api/items?page={(index % 20) + 1}&pageSize=48&q=%D0%B3%D0%BD%D0%B5%D0%B2&server={server}",
                )
            elif kind == 1:
                status, _ = json_request(
                    app.base_url,
                    f"/api/monsters?page={(index % 20) + 1}&pageSize=48&sort=level&server={server}",
                )
            elif kind == 2:
                item_id = (80567, 253, 80243)[index % 3]
                status, _ = json_request(
                    app.base_url, f"/api/items/{item_id}?server={server}"
                )
            elif kind == 3:
                monster_id = (10042, 141)[index % 2]
                status, _ = json_request(
                    app.base_url, f"/api/monsters/{monster_id}?server={server}"
                )
            elif kind == 4:
                status, _ = json_request(
                    app.base_url,
                    f"/api/search?q=%D0%B3%D0%BD%D0%B5%D0%B2%20%D0%BF%D1%80%D0%B5%D0%B4%D0%BA%D0%BE%D0%B2&server={server}",
                )
            elif kind == 5:
                status, _ = json_request(
                    app.base_url,
                    "/api/favorites",
                    method="POST",
                    payload={
                        "keys": favorite_keys,
                        "server": server,
                        "page": 1,
                        "pageSize": 50,
                    },
                )
            elif kind == 6:
                status, _ = json_request(app.base_url, "/api/user-data")
            else:
                profile = {
                    "schemaVersion": 1,
                    "migrated": True,
                    "settings": {
                        "server": server,
                        "theme": "dark" if server == "kiss" else "light",
                        "view": "list",
                    },
                    "itemFilters": {"q": "гнев", "sort": "name"},
                    "monsterFilters": {"sort": "level"},
                    "favorites": favorite_keys,
                    "history": ["гнев предков"],
                }
                status, _ = json_request(
                    app.base_url, "/api/user-data", method="PUT", payload=profile
                )
            if status != 200:
                raise AssertionError(
                    f"mixed request {index} failed with status {status}"
                )
        gc.collect()
        time.sleep(2.0)
        after = process.memory_info().rss
        delta = after - before
        print(f"RSS before: {before} bytes")
        print(f"RSS after:  {after} bytes")
        print(f"RSS delta:  {delta:+d} bytes")

        if delta > 64 * 1024 * 1024:
            raise AssertionError(f"RSS grew by more than 64 MiB: {delta}")

    print("RSS smoke test: PASS (not a leak proof)")


if __name__ == "__main__":
    main()
