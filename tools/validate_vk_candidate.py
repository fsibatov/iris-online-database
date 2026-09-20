import html
import json
import re
import sys
from pathlib import Path

BR_TAG = re.compile(r"<br\s*/?>", re.IGNORECASE)
HTML_TAG = re.compile(r"<[^>]+>")
DECORATIVE_SYMBOL = re.compile("[\u2600-\u27bf\U0001f300-\U0001faff\ufe0f\u200d]")
MARKUP = re.compile(r"[*_`~]+")


def validate(payload):
    if not isinstance(payload, dict):
        raise SystemExit(2)
    post_id = payload.get("post_id")
    post_url = payload.get("post_url")
    text = str(payload.get("text") or "").strip()
    if not isinstance(post_id, int) or isinstance(post_id, bool) or post_id <= 0:
        raise SystemExit(2)
    if post_url != f"https://vk.ru/wall-59626511_{post_id}":
        raise SystemExit(3)
    if not text:
        raise SystemExit(4)
    return post_id


def semantic_text(value):
    text = BR_TAG.sub("\n", str(value or ""))
    text = HTML_TAG.sub(" ", html.unescape(text))
    text = DECORATIVE_SYMBOL.sub("", text)
    text = MARKUP.sub("", text)
    return " ".join(text.split()).casefold()


IDENTITY_KEYS = (
    "schema",
    "community_url",
    "post_id",
    "post_url",
    "published_at",
)


def compare(current, candidate):
    current_id = validate(current)
    candidate_id = validate(candidate)
    same_identity = all(
        current.get(key, "") == candidate.get(key, "") for key in IDENTITY_KEYS
    )
    same_text = semantic_text(current.get("text")) == semantic_text(
        candidate.get("text")
    )
    if candidate_id < current_id:
        return candidate_id, "stale"
    if candidate_id > current_id:
        return candidate_id, "promote"
    if same_identity and same_text:
        return candidate_id, "same"
    return candidate_id, "promote"


def main():
    if len(sys.argv) != 3:
        raise SystemExit(2)
    current = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
    candidate = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
    print(*compare(current, candidate))


if __name__ == "__main__":
    main()
