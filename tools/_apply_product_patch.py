from __future__ import annotations

import base64
import gzip
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PATCH_B64 = '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'


def run(*args: str) -> None:
    subprocess.run(args, cwd=ROOT, check=True)


patch = gzip.decompress(base64.b64decode(PATCH_B64))
patch_path = ROOT / ".git" / "product-cleanup.patch"
patch_path.write_bytes(patch)
run("git", "apply", "--check", str(patch_path))
run("git", "apply", str(patch_path))

workflow_path = ROOT / ".github" / "workflows" / "update-vk-news.yml"
workflow = workflow_path.read_text(encoding="utf-8")
old = """import json
import sys
from pathlib import Path
"""
new = """import html
import json
import re
import sys
from pathlib import Path
"""
if workflow.count(old) != 1:
    raise SystemExit("VK workflow import block no longer matches expected source")
workflow = workflow.replace(old, new, 1)
old = """          comparable_keys = (
              \"schema\",
              \"community_url\",
              \"post_id\",
              \"post_url\",
              \"text\",
              \"published_at\",
          )

          if candidate_id < current_id:
              action = \"stale\"
          elif candidate_id > current_id:
              action = \"promote\"
          elif all(current.get(key, \"\") == candidate.get(key, \"\") for key in comparable_keys):
              action = \"same\"
          else:
              action = \"promote\"
"""
new = """          identity_keys = (
              \"schema\",
              \"community_url\",
              \"post_id\",
              \"post_url\",
              \"published_at\",
          )
          br_tag = re.compile(r\"<br\\s*/?>\", re.IGNORECASE)
          html_tag = re.compile(r\"<[^>]+>\")
          decorative_symbol = re.compile(
              \"[\\u2600-\\u27BF\\U0001F300-\\U0001FAFF\\uFE0F\\u200D]\"
          )
          markup = re.compile(r\"[*_`~]+\")

          def semantic_text(value):
              text = br_tag.sub(\"\\n\", str(value or \"\"))
              text = html_tag.sub(\" \", html.unescape(text))
              text = decorative_symbol.sub(\"\", text)
              text = markup.sub(\"\", text)
              return \" \".join(text.split()).casefold()

          same_identity = all(
              current.get(key, \"\") == candidate.get(key, \"\") for key in identity_keys
          )
          same_text = semantic_text(current.get(\"text\")) == semantic_text(
              candidate.get(\"text\")
          )

          if candidate_id < current_id:
              action = \"stale\"
          elif candidate_id > current_id:
              action = \"promote\"
          elif same_identity and same_text:
              action = \"same\"
          else:
              action = \"promote\"
"""
if workflow.count(old) != 1:
    raise SystemExit("VK workflow comparison block no longer matches expected source")
workflow_path.write_text(workflow.replace(old, new, 1), encoding="utf-8")

# Temporary transport files must never survive in the product diff.
run("git", "checkout", "origin/main", "--", ".github/workflows/dependency-review.yml")
Path(__file__).unlink()

run("python", "-B", "tools/validate_workflows.py")
run("node", "--check", "web/app.js")
run("git", "diff", "--check")
