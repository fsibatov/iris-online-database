"""Build transformation-card data from the authoritative Iris resource tables."""

from __future__ import annotations

import argparse
import gzip
import json
import re
from pathlib import Path

BASE_PLAYER_RUN_SPEED = 450
TRANSFORM_STATUS = 1002
MOVE_SPEED_PLUS_STATUS = 120
MOVE_SPEED_PERCENT_STATUS = 216


def _int(value: str, default: int = 0) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def read_rows(path: Path, encoding: str = "cp1251") -> dict[int, list[str]]:
    rows: dict[int, list[str]] = {}
    for raw in path.read_text(encoding=encoding, errors="replace").splitlines():
        if not raw or raw.startswith("//"):
            continue
        parts = raw.split("\t")
        index = _int(parts[0], -1) if parts else -1
        if index >= 0:
            rows[index] = parts
    return rows


def read_monster_skills(path: Path) -> dict[int, list[list[str]]]:
    result: dict[int, list[list[str]]] = {}
    for raw in path.read_text(encoding="cp1251", errors="replace").splitlines():
        if not raw or raw.startswith("//"):
            continue
        parts = raw.split("\t")
        if len(parts) < 29:
            continue
        monster_id = _int(parts[0], -1)
        slot = _int(parts[1], -1)
        if monster_id < 0 or slot not in {2, 3, 4}:
            continue
        result.setdefault(monster_id, []).append(parts)
    for skills in result.values():
        skills.sort(key=lambda row: _int(row[1]))
    return result


def read_texts(path: Path) -> dict[int, str]:
    result: dict[int, str] = {}
    for raw in path.read_text(encoding="utf-16", errors="strict").splitlines():
        if not raw or raw.startswith("//") or "\t" not in raw:
            continue
        key, value = raw.split("\t", 1)
        index = _int(key.strip(), -1)
        if index < 0:
            continue
        result[index] = (
            value.strip()
            .strip('"')
            .replace("\\r\\n", "\n")
            .replace("\\n", "\n")
            .replace("\\r", "\n")
        )
    return result


def read_gzip_json(path: Path) -> dict:
    with gzip.open(path, "rt", encoding="utf-8") as source:
        return json.load(source)


def effect_statuses(row: list[str] | None) -> list[dict[str, int]]:
    if not row:
        return []
    result: list[dict[str, int]] = []
    for index_pos, time_pos, value_pos in ((17, 18, 19), (20, 21, 22)):
        if len(row) <= value_pos:
            continue
        code = _int(row[index_pos])
        if code:
            result.append(
                {
                    "code": code,
                    "time": _int(row[time_pos]),
                    "value": _int(row[value_pos]),
                }
            )
    for index_pos, value_pos in ((23, 24), (25, 26)):
        if len(row) <= value_pos:
            continue
        code = _int(row[index_pos])
        if code:
            result.append({"code": code, "time": 0, "value": _int(row[value_pos])})
    return result


def transform_monster_id(row: list[str] | None) -> int:
    for status in effect_statuses(row):
        if status["code"] == TRANSFORM_STATUS and status["value"] > 0:
            return status["value"]
    return 0


def clean_template(value: str) -> str:
    return (
        str(value or "")
        .replace("\r\n", "\n")
        .replace("\r", "\n")
        .replace("%%", "%")
        .strip()
    )


def format_effect_text(template: str, statuses: list[dict[str, int]]) -> str:
    text = clean_template(template)
    if not text:
        return ""
    placeholders = len(re.findall(r"%d", text))
    values = [
        status["value"]
        for status in statuses
        if status["code"] != TRANSFORM_STATUS and status["value"] != 0
    ]
    if placeholders > len(values):
        values.extend(
            status["value"]
            for status in statuses
            if status["code"] != TRANSFORM_STATUS and status["value"] == 0
        )
    for value in values[:placeholders]:
        text = text.replace("%d", str(value), 1)
    text = re.sub(r"%[ds]", "", text)
    text = text.replace("%%", "%")
    text = re.sub(r"[ \t]+\n", "\n", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


CANONICAL_PATTERNS: tuple[tuple[re.Pattern[str], str], ...] = (
    (re.compile(r"скорост.*(?:бег|передвиж|движ)", re.I), "Скорость бега"),
    (re.compile(r"скорост.*атак", re.I), "Скорость атаки"),
    (
        re.compile(r"защит.*(?:понижен|снижен).*скорост", re.I),
        "Защита от снижения скорости",
    ),
    (re.compile(r"физ.*уклон", re.I), "Физическое уклонение"),
    (re.compile(r"маг.*уклон", re.I), "Магическое уклонение"),
    (re.compile(r"физ.*защит", re.I), "Физическая защита"),
    (re.compile(r"маг.*защит", re.I), "Магическая защита"),
    (re.compile(r"вс(?:е|ей|ем).*защит|ко всем защит", re.I), "Все виды защиты"),
    (re.compile(r"физ.*(?:поглощ|насыщ)", re.I), "Физическое поглощение"),
    (
        re.compile(r"(?:маг.*(?:поглощ|насыщ)|(?:поглощ|насыщ).*маг)", re.I),
        "Магическое поглощение",
    ),
    (re.compile(r"физ.*отраж", re.I), "Физическое отражение"),
    (re.compile(r"маг.*отраж", re.I), "Магическое отражение"),
    (re.compile(r"похищ.*здоров", re.I), "Похищение здоровья"),
    (re.compile(r"предел.*здоров|макс.*(?:оз|здоров)", re.I), "Максимум здоровья"),
    (re.compile(r"предел.*ман|макс.*(?:ом|ман)", re.I), "Максимум маны"),
    (re.compile(r"вас лечат", re.I), "Получаемое исцеление"),
    (re.compile(r"^лечение\b.*здоров", re.I), "Восстановление здоровья"),
    (re.compile(r"лечение отрав", re.I), "Лечение отравления"),
    (re.compile(r"регенерац", re.I), "Регенерация"),
    (re.compile(r"поток.*ман", re.I), "Поток маны"),
    (re.compile(r"физ.*метк", re.I), "Физическая меткость"),
    (re.compile(r"маг.*метк", re.I), "Магическая меткость"),
    (re.compile(r"(?:^|\s)метк", re.I), "Меткость"),
    (re.compile(r"физ.*стойк", re.I), "Физическая стойкость"),
    (re.compile(r"маг.*стойк", re.I), "Магическая стойкость"),
    (re.compile(r"физ.*ярост", re.I), "Физическая ярость"),
    (re.compile(r"маг.*ярост", re.I), "Магическая ярость"),
    (re.compile(r"урон.*удар|физ.*урон", re.I), "Физический урон"),
    (re.compile(r"урон.*выстр|сил.*выстр", re.I), "Урон выстрелов"),
    (re.compile(r"маг.*урон", re.I), "Магический урон"),
    (re.compile(r"(?:всему|общ).*урон", re.I), "Общий урон"),
    (re.compile(r"дальност.*атак", re.I), "Дальность атаки"),
    (re.compile(r"характеристик", re.I), "Все характеристики"),
    (re.compile(r"получ.*агресс", re.I), "Получение агрессии"),
    (re.compile(r"невидим.*щит|количеств.*щит", re.I), "Количество щитов"),
    (re.compile(r"поглощ.*урон.*щит", re.I), "Поглощение урона щитом"),
    (re.compile(r"защищает от любых атак", re.I), "Защита от любых атак"),
    (
        re.compile(r"разрушить защиту ивентового босса", re.I),
        "Разрушение защиты ивентового босса",
    ),
    (
        re.compile(r"снят.*(?:негатив|отриц).*эффект", re.I),
        "Снятие отрицательных эффектов",
    ),
    (re.compile(r"иммунитет к (?:яду|ядам)", re.I), "Иммунитет к ядам"),
    (re.compile(r"иммунитет к обездвиж", re.I), "Иммунитет к обездвиживанию"),
    (re.compile(r"\bсил[аеуы]?\b", re.I), "Сила"),
    (re.compile(r"ловкост", re.I), "Ловкость"),
    (re.compile(r"выносливост", re.I), "Выносливость"),
    (re.compile(r"интеллект", re.I), "Интеллект"),
    (re.compile(r"мудрост", re.I), "Мудрость"),
    (re.compile(r"размер|\bрост(?:а|у|ом)?\b", re.I), "Размер"),
)

NOISE_CHARACTERISTIC_PATTERNS: tuple[re.Pattern[str], ...] = (
    re.compile(r"^\[?npc\]?\s*превращение", re.I),
    re.compile(r"общий\s*/\s*подтип.*трансформац", re.I),
    re.compile(r"во время превращения", re.I),
    re.compile(r"число основной атаки", re.I),
    re.compile(r"^превращение критика", re.I),
    re.compile(r"^расходует ману вместо здоровья", re.I),
    re.compile(r"^пока ваши ом", re.I),
    re.compile(r"^от максимума", re.I),
    re.compile(r"^\[?увеличение\]?", re.I),
)


def canonical_characteristic(line: str) -> str:
    clean = line.strip().lstrip("*-• ").strip()
    for pattern, label in CANONICAL_PATTERNS:
        if pattern.search(clean):
            return label
    numeric_free = re.sub(r"[+−-]?\d+(?:[.,]\d+)?\s*%?", "", clean)
    numeric_free = re.sub(r"[\[\](){}]", " ", numeric_free)
    numeric_free = re.sub(r"^[\s:;,.+\-−]+|[\s:;,.+\-−]+$", "", numeric_free)
    numeric_free = re.sub(r"\s+", " ", numeric_free).strip()
    if numeric_free.lower().startswith("к "):
        numeric_free = numeric_free[2:].strip()
    return numeric_free[:1].upper() + numeric_free[1:] if numeric_free else ""


def characteristics(text: str) -> list[dict]:
    result: list[dict] = []
    seen: set[tuple[str, int | None, bool]] = set()
    for raw in clean_template(text).splitlines():
        line = raw.strip().lstrip("*• ").strip()
        if not line:
            continue
        lower = line.lower()
        if lower.startswith("превращение в") or lower.startswith("ты -"):
            continue
        if lower.startswith("наносит "):
            continue
        if any(pattern.search(line) for pattern in NOISE_CHARACTERISTIC_PATTERNS):
            continue
        match = re.search(r"(?<!\d)([+−-]?\d+(?:[.,]\d+)?)\s*(%)?", line)
        value: int | float | None = None
        percent = False
        if match:
            token = match.group(1).replace("−", "-").replace(",", ".")
            number = float(token)
            value = int(number) if number.is_integer() else number
            percent = bool(match.group(2))
        name = canonical_characteristic(line)
        if not name:
            continue
        key = (name.casefold(), value, percent)
        if key in seen:
            continue
        seen.add(key)
        result.append(
            {
                "name": name,
                "value": value,
                "percent": percent,
                "positive": value is None or value >= 0,
                "text": line,
            }
        )
    return result


def skill_name(
    skill_row: list[str],
    effect_row: list[str] | None,
    names: dict[int, str],
    tooltips: dict[int, str],
) -> str:
    if effect_row and len(effect_row) > 2:
        name = names.get(_int(effect_row[2]), "").strip()
        if name:
            return name
    tooltip = tooltips.get(_int(skill_row[3]), "").strip() if len(skill_row) > 3 else ""
    if tooltip:
        return tooltip.splitlines()[0].strip()
    return f"Навык {max(1, _int(skill_row[1]) - 1)}"


def build(
    game_data: Path,
    item_abilities: Path,
    monster_list: Path,
    monster_skills: Path,
    skill_effects: Path,
    skill_names: Path,
    skill_tooltips: Path,
) -> dict:
    game = read_gzip_json(game_data)
    abilities = read_gzip_json(item_abilities).get("items", {})
    effects = read_rows(skill_effects)
    monsters = read_rows(monster_list)
    skills_by_monster = read_monster_skills(monster_skills)
    names = read_texts(skill_names)
    tooltips = read_texts(skill_tooltips)

    cards: list[dict] = []
    for item in game.get("items", []):
        if item.get("middleCategory") != "Карта монстра":
            continue
        patch = abilities.get(str(item.get("id")), {})
        influence_id = int(patch.get("influenceIndex") or 0)
        effect_row = effects.get(influence_id)
        monster_id = transform_monster_id(effect_row)
        monster_row = monsters.get(monster_id)
        if influence_id <= 0 or monster_id <= 0 or not effect_row or not monster_row:
            continue

        form_statuses = effect_statuses(effect_row)
        form_template = (
            tooltips.get(_int(effect_row[4]), "") if len(effect_row) > 4 else ""
        )
        form_text = format_effect_text(form_template, form_statuses)
        run_speed = _int(monster_row[29]) if len(monster_row) > 29 else 0
        move_plus = sum(
            status["value"]
            for status in form_statuses
            if status["code"] == MOVE_SPEED_PLUS_STATUS
        )
        move_percent = sum(
            status["value"]
            for status in form_statuses
            if status["code"] == MOVE_SPEED_PERCENT_STATUS
        )
        multiplier = max(1, 100 + move_percent) / 100.0
        effective_speed = (
            max(0.0, run_speed * multiplier + move_plus) if run_speed > 1 else 0.0
        )
        if effective_speed.is_integer():
            effective_speed = int(effective_speed)
        delta = effective_speed - BASE_PLAYER_RUN_SPEED
        if isinstance(delta, float) and delta.is_integer():
            delta = int(delta)
        delta_percent = round(float(delta) * 100.0 / BASE_PLAYER_RUN_SPEED, 1)

        skill_rows: list[dict] = []
        for skill_row in skills_by_monster.get(monster_id, []):
            effect_id = _int(skill_row[28]) if len(skill_row) > 28 else 0
            skill_effect = effects.get(effect_id)
            statuses = effect_statuses(skill_effect)
            template = (
                tooltips.get(_int(skill_effect[4]), "")
                if skill_effect and len(skill_effect) > 4
                else ""
            )
            effect_text = format_effect_text(template, statuses)
            apply_type = _int(skill_row[22]) if len(skill_row) > 22 else 0
            skill_type = _int(skill_row[4]) if len(skill_row) > 4 else 0
            skill_rows.append(
                {
                    "position": max(1, _int(skill_row[1]) - 1),
                    "name": skill_name(skill_row, skill_effect, names, tooltips),
                    "skillTooltipIndex": _int(skill_row[3])
                    if len(skill_row) > 3
                    else 0,
                    "effectId": effect_id,
                    "cooldownMs": max(
                        0, _int(skill_row[15]) if len(skill_row) > 15 else 0
                    ),
                    "manaCost": max(
                        0, _int(skill_row[12]) if len(skill_row) > 12 else 0
                    ),
                    "durationMs": max(
                        0,
                        _int(skill_effect[15])
                        if skill_effect and len(skill_effect) > 15
                        else 0,
                    ),
                    "skillType": skill_type,
                    "applyType": apply_type,
                    "target": {1: "На себя", 2: "На противника", 3: "На союзника"}.get(
                        apply_type, ""
                    ),
                    "effectText": effect_text,
                    "characteristics": characteristics(effect_text),
                    "isSelfBuff": bool(
                        effect_text and apply_type == 1 and skill_type == 2
                    ),
                    "isFriendlyBuff": bool(
                        effect_text and apply_type in {1, 3} and skill_type in {2, 3}
                    ),
                }
            )

        form_characteristics = characteristics(form_text)
        buff_names = {row["name"] for row in form_characteristics if row["positive"]}
        for skill in skill_rows:
            if not skill["isFriendlyBuff"]:
                continue
            buff_names.update(
                row["name"] for row in skill["characteristics"] if row["positive"]
            )

        cards.append(
            {
                "itemId": int(item["id"]),
                "name": str(item.get("name") or "").strip(),
                "quality": str(item.get("quality") or "").strip(),
                "qualityId": int(item.get("qualityId") or 0),
                "monsterId": monster_id,
                "formName": names.get(_int(effect_row[2]), "").strip()
                if len(effect_row) > 2
                else "",
                "durationMs": max(
                    0, _int(effect_row[15]) if len(effect_row) > 15 else 0
                ),
                "runSpeed": run_speed,
                "effectiveRunSpeed": effective_speed,
                "speedDelta": delta,
                "speedDeltaPercent": delta_percent,
                "formEffectText": form_text,
                "formCharacteristics": form_characteristics,
                "buffs": sorted(buff_names, key=str.casefold),
                "skills": skill_rows,
            }
        )

    cards.sort(key=lambda row: (str(row["name"]).casefold(), int(row["itemId"])))
    return {
        "schemaVersion": 1,
        "basePlayerRunSpeed": BASE_PLAYER_RUN_SPEED,
        "cards": cards,
    }


def write_gzip_json(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(
        data, ensure_ascii=False, separators=(",", ":"), sort_keys=False
    ).encode("utf-8")
    with (
        path.open("wb") as output,
        gzip.GzipFile(
            filename="", mode="wb", fileobj=output, mtime=0, compresslevel=9
        ) as compressed,
    ):
        compressed.write(payload)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--game-data", type=Path, required=True)
    parser.add_argument("--item-abilities", type=Path, required=True)
    parser.add_argument("--monster-list", type=Path, required=True)
    parser.add_argument("--monster-skills", type=Path, required=True)
    parser.add_argument("--skill-effects", type=Path, required=True)
    parser.add_argument("--skill-names", type=Path, required=True)
    parser.add_argument("--skill-tooltips", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = build(
        args.game_data,
        args.item_abilities,
        args.monster_list,
        args.monster_skills,
        args.skill_effects,
        args.skill_names,
        args.skill_tooltips,
    )
    write_gzip_json(args.output, data)
    buff_names = {name for card in data["cards"] for name in card.get("buffs", [])}
    print(
        f"cards={len(data['cards'])} buffCharacteristics={len(buff_names)} output={args.output}"
    )


if __name__ == "__main__":
    main()
