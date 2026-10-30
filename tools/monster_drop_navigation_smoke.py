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
    # Primary Iris Resourses.rar, both server variants: monster 104,
    # item_dropn.txt line 568, group 10431 weight 42 (0.0042%);
    # item_droplist.txt group 10431, item 131651 weight 833 (0.0833%).
    # Exact per-attempt chance: 0.0000034986%, not 0.0000035%.
    tiny_monster = {'monster': {'id': 104, 'name': 'Маленький червь', 'category': 'Монстр',
                                 'typeName': 'Обычный', 'level': 43, 'aggressive': False},
                    'slots': [{'sourceLine': 568, 'choices': [
                        {'title': 'Группа 10431', 'groupId': 10431, 'baseSelectionChance': 0.0042,
                         'items': [{'itemId': 131651, 'item': 'Чёрные перчатки ренегата',
                                    'baseSelectionChance': 0.0833,
                                    'baseAttemptChance': 0.0000034986, 'quantity': 1}]}]}]}
    tiny_item = {'item': {'id': 131651, 'name': 'Чёрные перчатки ренегата',
                           'category': 'Экипировка', 'typeLine': 'Перчатки',
                           'quality': 'Обычное', 'qualityId': 1},
                 'bonuses': [], 'drops': []}
    # Primary Iris Resourses.rar, Kiss monster 10026: source lines 1327/1328,
    # the same item 868026 occurs in group 1002607 in two distinct slots.
    # Selecting one source must not mark the other source as the active row.
    cross_slot_monster = {'monster': {'id': 10026, 'name': 'Монстр 10026',
                                      'category': 'Босс', 'typeName': 'Монстр',
                                      'level': 70, 'aggressive': True},
                          'slots': [{'sourceLine': line, 'choices': [
                              {'title': 'Группа 1002607', 'groupId': 1002607,
                               'baseSelectionChance': 80,
                               'items': [{'itemId': 868026, 'item': 'Серебряный глаз',
                                          'baseSelectionChance': 100,
                                          'baseAttemptChance': 80, 'quantity': 1}]}]}
                              for line in (1327, 1328)]}
    cross_slot_item = {'item': {'id': 868026, 'name': 'Серебряный глаз',
                                 'category': 'Расходники', 'typeLine': 'Материал',
                                 'quality': 'Обычное', 'qualityId': 2},
                       'bonuses': [], 'drops': []}
    # Primary Iris Resourses.rar, both servers: monster 148, source line 775,
    # group 14801, item 835243 twice: position 2 gives x1 (line 10894),
    # position 3 gives x5 (line 10895). Quantity must remain visible on
    # each item's source row, not only on the monster drop group page.
    quantity_monster = {'monster': {'id': 148, 'name': 'Монстр 148', 'category': 'Монстр',
                                    'typeName': 'Обычный', 'level': 65},
                        'slots': [{'sourceLine': 775, 'choices': [
                            {'title': 'Группа 14801', 'groupId': 14801,
                             'baseSelectionChance': 100, 'items': [
                                 {'itemId': 835108, 'item': 'Предмет 835108', 'position': 1,
                                  'baseSelectionChance': 2, 'baseAttemptChance': 2, 'quantity': 1},
                                 {'itemId': 835243, 'item': 'Предмет 835243', 'position': 2,
                                  'baseSelectionChance': 1, 'baseAttemptChance': 1, 'quantity': 1},
                                 {'itemId': 835243, 'item': 'Предмет 835243', 'position': 3,
                                  'baseSelectionChance': 0.1, 'baseAttemptChance': 0.1, 'quantity': 5}]}]}]}
    quantity_item = {'item': {'id': 835243, 'name': 'Предмет 835243', 'category': 'Тест',
                              'typeLine': 'Предмет', 'quality': 'Обычное', 'qualityId': 1},
                     'bonuses': [], 'drops': [
                         {'itemId': 835243, 'monsterId': 148, 'monster': 'Монстр 148',
                          'source': 'Выпадение монстра', 'sourceLine': 775,
                          'choicePosition': 1, 'itemPosition': position, 'groupId': 14801,
                          'slotTitle': 'Попытка выпадения №1', 'groupChanceKnown': True,
                          'groupBaseChance': 100, 'itemBaseChance': chance,
                          'baseAttemptChance': chance, 'quantity': quantity}
                         for position, chance, quantity in ((2, 1, 1), (3, 0.1, 5))]}
    profile = {'schemaVersion': 1, 'migrated': True,
               'settings': {'server': 'kiss', 'theme': 'dark', 'view': 'list'},
               'itemFilters': {}, 'monsterFilters': {}, 'favorites': [], 'history': [], 'recentlyViewed': []}
    meta = {'servers': [{'key': 'kiss', 'name': 'Iris Kiss Kiss'}], 'effectSpecs': {}}
    responses = {'/api/user-data': profile, '/api/meta': meta, '/api/monsters/42': monster,
                 '/api/items/1002': item, '/api/monsters/10042': real_monster,
                 '/api/items/809022': real_item, '/api/monsters/8': duplicate_monster,
                 '/api/items/180201': duplicate_item, '/api/monsters/135': rare_monster,
                 '/api/items/2406': rare_item, '/api/monsters/104': tiny_monster,
                 '/api/items/131651': tiny_item, '/api/monsters/10026': cross_slot_monster,
                 '/api/items/868026': cross_slot_item, '/api/monsters/148': quantity_monster,
                 '/api/items/835243': quantity_item, '/api/update-check': {'checked': True, 'updateAvailable': False}}
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
            assert page.locator('[data-drop-group-host="1"] a[href="#item/1002"]').get_attribute('aria-current') is None, 'unselected source group incorrectly highlighted'
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
            # Primary RAR: line 2483 selects group 6000501 at 100% then 50%.
            # The second choice has a known 0% effective chance, not unknown odds.
            zero_source = page.locator('.source-list a[href^="#monster/10042"]')
            assert zero_source.locator('.source-chance strong').inner_text() == '0%', 'known-zero source chance rendered as missing'
            page.locator('.source-list a[href^="#monster/10042"]').click()
            page.locator('.detail-page[data-route="monster/10042"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group="1"]').get_attribute('open') is not None, 'exact primary-source choice not expanded'
            assert 'Шанс группы: 0%' in page.locator('[data-drop-group="1"] summary small').inner_text(), 'known-zero group chance rendered as missing'
            assert 'за одну основную попытку: 0%' in page.locator('[data-drop-group-host="1"] a small').inner_text(), 'known-zero item chance rendered as missing'
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
            page.evaluate("window.location.hash = '#monster/104'")
            page.locator('.detail-page[data-route="monster/104"]').wait_for(timeout=8000)
            page.locator('.lazy-monster-drops > summary').click()
            page.locator('[data-drop-group="0"] summary').click()
            tiny_chance = page.locator('[data-drop-group-host="0"] a[href="#item/131651"] small').inner_text()
            assert '0,0000034986%' in tiny_chance, f'primary-source tiny chance truncated: {tiny_chance}'
            page.locator('[data-drop-group-host="0"] a[href="#item/131651"]').click()
            page.locator('.detail-page[data-route="item/131651"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/104"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group="0"]').get_attribute('open') is not None
            assert page.locator('[data-drop-group-host="0"] a[href="#item/131651"]').get_attribute('aria-current') == 'true'
            page.evaluate("window.location.hash = '#monster/10026'")
            page.locator('.detail-page[data-route="monster/10026"]').wait_for(timeout=8000)
            page.locator('.lazy-monster-drops > summary').click()
            page.locator('[data-drop-group="0"] summary').click()
            page.locator('[data-drop-group="1"] summary').click()
            page.locator('[data-drop-group-host="1"] a[href="#item/868026"]').click()
            page.locator('.detail-page[data-route="item/868026"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/10026"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group="0"]').get_attribute('open') is not None
            assert page.locator('[data-drop-group="1"]').get_attribute('open') is not None
            assert page.locator('[data-drop-group-host="1"] a[aria-current="true"]').count() == 1, 'chosen primary source not highlighted'
            assert page.locator('[data-drop-group-host="0"] a[aria-current="true"]').count() == 0, 'unselected primary source highlighted'
            page.evaluate("window.location.hash = '#item/835243'")
            page.locator('.detail-page[data-route="item/835243"]').wait_for(timeout=8000)
            page.locator('[data-open-details="item-sources"]').click()
            quantity_sources = page.locator('.source-list a[href^="#monster/148?"]')
            assert quantity_sources.count() == 2, 'source quantity variants collapsed'
            assert 'Количество: ×5' not in quantity_sources.nth(0).inner_text(), 'single quantity mislabelled'
            assert 'Количество: ×5' in quantity_sources.nth(1).inner_text(), 'five-unit primary source quantity omitted'
            assert quantity_sources.nth(1).get_attribute('href').endswith('&position=3'), 'quantity source position lost'
            quantity_sources.nth(1).click()
            page.locator('.detail-page[data-route="monster/148"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group="0"]').get_attribute('open') is not None, 'quantity source group not expanded'
            selected = page.locator('[data-drop-group-host="0"] a[aria-current="true"]')
            assert selected.count() == 1 and selected.get_attribute('data-drop-item-position') == '3', 'wrong quantity source selected'
            assert '×5' in selected.inner_text(), 'quantity missing on monster group row'
            selected.click()
            page.locator('.detail-page[data-route="item/835243"]').wait_for(timeout=8000)
            page.locator('.detail-page [data-route-back]').click()
            page.locator('.detail-page[data-route="monster/148"]').wait_for(timeout=8000)
            assert page.locator('[data-drop-group-host="0"] a[data-drop-item-position="3"]').get_attribute('aria-current') == 'true', 'quantity branch lost on return'
            assert not errors, f'frontend JavaScript exceptions: {errors}'
        finally:
            browser.close()
    print('Monster drop navigation smoke: PASS')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
