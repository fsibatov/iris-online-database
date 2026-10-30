"""Browser regression: preserve monster drop expansion and highlight the viewed item.

Runs against the embedded HTML/JS with deterministic offline API fixtures, without
opening a TCP server or accessing the network. Requires Python Playwright/Chromium.
"""
from __future__ import annotations

import re
from pathlib import Path

from playwright.sync_api import sync_playwright

from frontend_smoke_test import launch_browser

ROOT = Path(__file__).resolve().parents[1]


def main() -> int:
    web = ROOT / 'web'
    html = (web / 'index.html').read_text(encoding='utf-8')
    html = re.sub(r'<meta http-equiv="Content-Security-Policy"[^>]*>', '', html)
    html = re.sub(r'<link rel="stylesheet" href="/styles.css">', '', html)
    html = re.sub(r'<script src="/app.js" defer></script>', '', html)
    items = [{'itemId': 1000 + i, 'item': f'Предмет {i}', 'baseSelectionChance': 2.4,
              'baseAttemptChance': 2.4, 'quantity': 1} for i in range(1, 37)]
    monster = {'monster': {'id': 42, 'name': 'Тестовый монстр', 'category': 'Монстр',
                           'typeName': 'Босс', 'level': 70, 'aggressive': False, 'hp': 1500},
               'slots': [{'choices': [{'title': 'Группа 1', 'baseSelectionChance': 100, 'items': items},
                                      {'title': 'Группа 1', 'baseSelectionChance': 0,
                                       'items': [{'itemId': 1002, 'item': 'Предмет 2', 'baseAttemptChance': 0}]}]}]}
    item = {'item': {'id': 1002, 'name': 'Предмет 2', 'category': 'Тест',
                     'typeLine': 'Предмет', 'quality': 'Обычное', 'qualityId': 1},
            'bonuses': [], 'drops': []}
    # Primary Iris Resourses.rar: Kiss monster 10042, line 2483, repeated
    # group 6000501 (100% then 50%), item 809022.
    real_monster = {'monster': {'id': 10042, 'name': 'Шару Гаррулл', 'category': 'Босс',
                                'typeName': 'Нежить', 'level': 89, 'aggressive': True, 'hp': 24166667},
                    'slots': [{'sourceLine': 2483, 'choices': [
                        {'title': 'Группа 6000501', 'groupId': 6000501, 'baseSelectionChance': 100,
                         'items': [{'itemId': 809022, 'item': 'Эскудо', 'baseSelectionChance': 100,
                                    'baseAttemptChance': 100, 'quantity': 1}]},
                        {'title': 'Группа 6000501', 'groupId': 6000501, 'baseSelectionChance': 0,
                         'items': [{'itemId': 809022, 'item': 'Эскудо', 'baseSelectionChance': 100,
                                    'baseAttemptChance': 0, 'quantity': 1}]}]}]}
    real_item = {'item': {'id': 809022, 'name': 'Эскудо', 'category': 'Ценности',
                          'typeLine': '[Ценности] Валюта', 'quality': 'Редкое', 'qualityId': 4},
                 'bonuses': [], 'drops': [
                     {'itemId': 809022, 'monsterId': 10042, 'monster': 'Шару Гаррулл',
                      'source': 'Выпадение монстра', 'sourceLine': 2483, 'slotNumber': 1,
                      'choicePosition': 2, 'groupId': 6000501, 'groupChanceKnown': True,
                      'groupBaseChance': 0, 'itemBaseChance': 100, 'baseAttemptChance': 0}]}
    # Primary Kiss tables: monster 8 (Укусеница), line 42, choice 3,
    # group 813 contains item 180201 twice, at positions 5 and 6.
    duplicate_monster = {'monster': {'id': 8, 'name': 'Укусеница', 'category': 'Монстр',
                                     'typeName': 'Обычный', 'level': 1, 'aggressive': False},
                         'slots': [{'sourceLine': 42, 'choices': [
                             {'title': 'Группа 811', 'baseSelectionChance': 20, 'items': []},
                             {'title': 'Группа 812', 'baseSelectionChance': 20, 'items': []},
                             {'title': 'Группа 813', 'baseSelectionChance': 20, 'items': [
                                 {'itemId': 160201, 'item': 'Предмет A', 'position': 1, 'baseAttemptChance': 0.075},
                                 {'itemId': 160202, 'item': 'Предмет B', 'position': 2, 'baseAttemptChance': 0.075},
                                 {'itemId': 170201, 'item': 'Предмет C', 'position': 3, 'baseAttemptChance': 0.075},
                                 {'itemId': 170202, 'item': 'Предмет D', 'position': 4, 'baseAttemptChance': 0.075},
                                 {'itemId': 180201, 'item': 'Перчатки путешественника', 'position': 5, 'baseAttemptChance': 0.075},
                                 {'itemId': 180201, 'item': 'Перчатки путешественника', 'position': 6, 'baseAttemptChance': 0.075},
                                 {'itemId': 190201, 'item': 'Предмет E', 'position': 7, 'baseAttemptChance': 0.075},
                                 {'itemId': 190202, 'item': 'Предмет F', 'position': 8, 'baseAttemptChance': 0.075},
                             ]}]}]}
    duplicate_item = {'item': {'id': 180201, 'name': 'Перчатки путешественника',
                               'category': 'Экипировка', 'typeLine': 'Перчатки',
                               'quality': 'Обычное', 'qualityId': 1},
                      'bonuses': [], 'drops': [
                          {'itemId': 180201, 'monsterId': 8, 'monster': 'Укусеница',
                           'source': 'Выпадение монстра', 'sourceLine': 42, 'choicePosition': 3,
                           'itemPosition': pos, 'groupId': 813, 'groupChanceKnown': True,
                           'groupBaseChance': 20, 'itemBaseChance': 0.375,
                           'baseAttemptChance': 0.075} for pos in (5, 6)]}
    # Real primary-source probability, both Kiss and Original: monster 135,
    # item_dropn.txt line 714, group 13531; item_droplist.txt line 9843,
    # item 2406 (raw weights 2000 and 625 => 0.000125% per attempt).
    rare_monster = {'monster': {'id': 135, 'name': 'Флерания', 'category': 'Особый',
                                'typeName': 'Растение', 'level': 65, 'aggressive': True},
                    'slots': [{'sourceLine': 714, 'choices': [
                        {'title': 'Группа 13531', 'groupId': 13531, 'baseSelectionChance': 0.2,
                         'items': [{'itemId': 2406, 'item': 'Спата золотого бойца',
                                    'baseSelectionChance': 0.0625,
                                    'baseAttemptChance': 0.000125, 'quantity': 1}]}]}]}
    rare_item = {'item': {'id': 2406, 'name': 'Спата золотого бойца',
                          'category': 'Оружие/щит', 'typeLine': '[Оружие] Меч',
                          'quality': 'Эпическое', 'qualityId': 7},
                 'bonuses': [], 'drops': []}
    profile = {'schemaVersion': 1, 'migrated': True,
               'settings': {'server': 'kiss', 'theme': 'dark', 'view': 'list'},
               'itemFilters': {}, 'monsterFilters': {}, 'favorites': [], 'history': [], 'recentlyViewed': []}
    meta = {'servers': [{'key': 'kiss', 'name': 'Iris Kiss Kiss'}], 'effectSpecs': {}}
    responses = {'/api/user-data': profile, '/api/meta': meta, '/api/monsters/42': monster,
                 '/api/items/1002': item, '/api/monsters/10042': real_monster,
                 '/api/items/809022': real_item, '/api/monsters/8': duplicate_monster,
                 '/api/items/180201': duplicate_item, '/api/monsters/135': rare_monster,
                 '/api/items/2406': rare_item, '/api/update-check': {'checked': True, 'updateAvailable': False}}
    for item_id in (1001, 1003):
        responses[f'/api/items/{item_id}'] = {**item, 'item': {**item['item'], 'id': item_id, 'name': f'Предмет {item_id - 1000}'}}
    with sync_playwright() as p:
        browser = launch_browser(p)
        try:
            page = browser.new_page(viewport={'width': 950, 'height': 700})
            errors = []
            page.on('pageerror', lambda e: errors.append(str(e)))
            page.goto('about:blank')
            page.set_content(html, wait_until='load')
            page.add_style_tag(content=(web / 'styles.css').read_text(encoding='utf-8'))
            page.evaluate('''(responses) => {
              const entries = new Map();
              Object.defineProperty(window, 'localStorage', {configurable:true, value:{
                getItem:k=>entries.has(k)?entries.get(k):null,
                setItem:(k,v)=>entries.set(k,String(v)), removeItem:k=>entries.delete(k)
              }});
              window.fetch = async (url) => {
                const path = String(url).split('?')[0];
                const result = responses[path];
                if (!result) return {ok:false,status:404,json:async()=>({})};
                return {ok:true,status:200,json:async()=>JSON.parse(JSON.stringify(result))};
              };
              window.history.replaceState({}, '', '#monster/42');
              window.go = {main:{DesktopBridge:{OpenExternalURL:async()=>{}}}};
            }''', responses)
            page.add_script_tag(content=(web / 'app.js').read_text(encoding='utf-8'))
            page.locator('.lazy-monster-drops').wait_for(timeout=8000)
            page.locator('[data-open-details="monster-drops"]').click()
            page.locator('[data-drop-group="0"] summary').click()
            page.locator('[data-drop-group="1"] summary').click()
            # Distinct source positions must remain distinct even when group IDs repeat.
            assert page.locator('[data-drop-group]').count() == 2, 'duplicate source group collapsed'
            assert page.locator('[data-drop-group="0"] summary strong').inner_text() == page.locator('[data-drop-group="1"] summary strong').inner_text()
            page.locator('[data-drop-more="0"]').click()
            assert page.locator('[data-drop-group-host="0"] a').count() == 36, 'expanded before click'
            page.locator('[data-drop-group-host="0"] a[href="#item/1002"]').click()
            page.locator('.detail-page[data-route="item/1002"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/42"]').wait_for(timeout=8000)
            page.wait_for_timeout(220)
            assert page.locator('.lazy-monster-drops').get_attribute('open') is not None, 'monster drops collapsed on return'
            assert page.locator('[data-drop-group="0"]').get_attribute('open') is not None, 'selected drop group collapsed on return'
            assert page.locator('[data-drop-group="1"]').get_attribute('open') is not None, 'other expanded group collapsed on return'
            assert page.locator('[data-drop-group-host="0"] a').count() == 36, 'loaded drop items reset on return'
            selected = page.locator('[data-drop-group-host="0"] a[href="#item/1002"]')
            assert 'is-active-drop' in (selected.get_attribute('class') or '').split(), 'active item not highlighted'
            assert selected.get_attribute('aria-current') == 'true', 'active item has no accessible indication'
            assert selected.evaluate('(node) => getComputedStyle(node).boxShadow') != 'none', 'active item has no visible accent'
            saved_scroll, current_scroll = page.evaluate('''() => [history.state.monsterDropView.scrollY, window.scrollY]''')
            assert abs(saved_scroll - current_scroll) < 24, f'drop scroll position not restored: {saved_scroll} -> {current_scroll}'
            assert page.locator('[data-drop-group-host="1"] a[href="#item/1002"]').get_attribute('aria-current') == 'true', 'same item in another group not highlighted'
            assert page.locator('[data-drop-group-host="0"] a[href="#item/1003"]').get_attribute('aria-current') is None, 'unselected row highlighted'
            page.locator('[data-drop-group-host="0"] a[href="#item/1003"]').click()
            page.locator('.detail-page[data-route="item/1003"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/42"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group-host="0"] a[href="#item/1003"]').get_attribute('aria-current') == 'true', 'subsequently viewed item not selected'
            assert page.locator('[data-drop-group-host="0"] a[href="#item/1002"]').get_attribute('aria-current') is None, 'previous selection persisted'
            page.locator('.lazy-monster-drops > summary').click()  # Closed at selection time.
            page.locator('.drop-preview-list a[href="#item/1001"]').click()
            page.locator('.detail-page[data-route="item/1001"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/42"]').wait_for(timeout=8000)
            assert page.locator('.lazy-monster-drops').get_attribute('open') is None, 'intentionally closed drop section reopened'
            assert page.locator('.drop-preview-list a[href="#item/1001"]').get_attribute('aria-current') == 'true', 'preview selection not highlighted'
            page.evaluate("document.documentElement.dataset.theme = 'light'")
            assert page.locator('.drop-preview-list a[href="#item/1001"]').evaluate('(node) => getComputedStyle(node).boxShadow') != 'none', 'active preview not visible in light theme'
            # Item source must navigate to its exact source slot and choice, not
            # merely to the monster header. This is an actual primary-source ID.
            page.evaluate("window.location.hash = '#item/809022'")
            page.locator('.detail-page[data-route="item/809022"]').wait_for(timeout=8000)
            page.locator('[data-open-details="item-sources"]').click()
            page.locator('.source-list a[href^="#monster/10042"]').click()
            page.locator('.detail-page[data-route="monster/10042"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group="1"]').get_attribute('open') is not None, 'exact primary-source choice not expanded'
            assert page.locator('[data-drop-group-host="1"] a[href="#item/809022"]').get_attribute('aria-current') == 'true', 'source item not selected'
            page.locator('[data-drop-group-host="1"] a[href="#item/809022"]').click()
            page.locator('.detail-page[data-route="item/809022"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/10042"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group="1"]').get_attribute('open') is not None, 'source group collapsed after return'
            assert page.locator('[data-drop-group-host="1"] a[href="#item/809022"]').get_attribute('aria-current') == 'true', 'source item highlight lost after return'
            # Duplicate item rows must navigate to their exact primary-source
            # position, not silently select the first identical item ID.
            page.evaluate("window.location.hash = '#item/180201'")
            page.locator('.detail-page[data-route="item/180201"]').wait_for(timeout=8000)
            page.locator('[data-open-details="item-sources"]').click()
            links = page.locator('.source-list a[href^="#monster/8?"]')
            assert links.count() == 2, 'primary-source duplicate rows collapsed'
            assert links.nth(1).get_attribute('href') == '#monster/8?item=180201&line=42&choice=3&position=6', 'duplicate row source position lost'
            links.nth(1).click()
            page.locator('.detail-page[data-route="monster/8"]').wait_for(timeout=8000)
            selected = page.locator('[data-drop-group-host="2"] a[aria-current="true"]')
            assert selected.count() == 1, 'duplicate item rows both highlighted'
            assert selected.get_attribute('data-drop-item-position') == '6', 'wrong duplicate item row selected'
            selected.click()
            page.locator('.detail-page[data-route="item/180201"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/8"]').wait_for(timeout=8000)
            selected = page.locator('[data-drop-group-host="2"] a[aria-current="true"]')
            assert selected.count() == 1 and selected.get_attribute('data-drop-item-position') == '6', 'duplicate selection not restored'
            page.evaluate("window.location.hash = '#monster/135'")
            page.locator('.detail-page[data-route="monster/135"]').wait_for(timeout=8000)
            page.locator('.lazy-monster-drops > summary').click()
            page.locator('[data-drop-group="0"] summary').click()
            chance_text = page.locator('[data-drop-group-host="0"] a[href="#item/2406"] small').inner_text()
            assert '0,000125%' in chance_text, f'primary-source rare chance rounded inaccurately: {chance_text}'
            page.locator('[data-drop-group-host="0"] a[href="#item/2406"]').click()
            page.locator('.detail-page[data-route="item/2406"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/135"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group="0"]').get_attribute('open') is not None
            assert page.locator('[data-drop-group-host="0"] a[href="#item/2406"]').get_attribute('aria-current') == 'true'
            assert not errors, f'frontend JavaScript exceptions: {errors}'
        finally:
            browser.close()
    print('Monster drop navigation smoke: PASS')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
