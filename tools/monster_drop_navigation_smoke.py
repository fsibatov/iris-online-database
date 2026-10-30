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
    profile = {'schemaVersion': 1, 'migrated': True,
               'settings': {'server': 'kiss', 'theme': 'dark', 'view': 'list'},
               'itemFilters': {}, 'monsterFilters': {}, 'favorites': [], 'history': [], 'recentlyViewed': []}
    meta = {'servers': [{'key': 'kiss', 'name': 'Iris Kiss Kiss'}], 'effectSpecs': {}}
    responses = {'/api/user-data': profile, '/api/meta': meta, '/api/monsters/42': monster,
                 '/api/items/1002': item, '/api/update-check': {'checked': True, 'updateAvailable': False}}
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
            assert not errors, f'frontend JavaScript exceptions: {errors}'
        finally:
            browser.close()
    print('Monster drop navigation smoke: PASS')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
