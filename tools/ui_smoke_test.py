#!/usr/bin/env python3
"""Chromium UI smoke test for accessibility, responsiveness and lazy DOM."""
from __future__ import annotations

import argparse
import re
import time
import urllib.error
import urllib.request
from urllib.parse import urlsplit

from playwright.sync_api import sync_playwright

from smoke_common import RunningApp, require_binary


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    binary = require_binary(args.binary)
    js_errors: list[str] = []

    with RunningApp(binary, ["-no-browser"]) as app:
        app.wait_ready()
        with sync_playwright() as playwright:
            browser = playwright.chromium.launch(executable_path="/usr/bin/chromium", headless=True, args=["--no-sandbox", "--disable-dev-shm-usage"])
            page = browser.new_page(viewport={"width": 375, "height": 900})
            def proxy_request(route):
                request = route.request
                parsed = urlsplit(request.url)
                target = app.base_url + parsed.path + (("?" + parsed.query) if parsed.query else "")
                if parsed.path.startswith("/api/items/"):
                    time.sleep(0.015)
                headers = {"Accept": request.headers.get("accept", "*/*")}
                if request.headers.get("content-type"):
                    headers["Content-Type"] = request.headers["content-type"]
                upstream = urllib.request.Request(target, data=request.post_data_buffer, headers=headers, method=request.method)
                try:
                    with urllib.request.urlopen(upstream, timeout=10) as response:
                        response_headers = {key: value for key, value in response.headers.items() if key.lower() not in {"content-encoding", "transfer-encoding", "connection"}}
                        route.fulfill(status=response.status, headers=response_headers, body=response.read())
                except urllib.error.HTTPError as error:
                    route.fulfill(status=error.code, headers={"Content-Type": error.headers.get("Content-Type", "text/plain")}, body=error.read())

            page.route("http://iris.test/**", proxy_request)
            page.on("pageerror", lambda error: js_errors.append(str(error)))
            page.evaluate("""() => {
              const values = new Map();
              Object.defineProperty(window, 'localStorage', {
                configurable: true,
                value: {
                  getItem: key => values.has(String(key)) ? values.get(String(key)) : null,
                  setItem: (key, value) => values.set(String(key), String(value)),
                  removeItem: key => values.delete(String(key)),
                  clear: () => values.clear(),
                },
              });
            }""")
            with urllib.request.urlopen(app.base_url + "/", timeout=10) as response:
                html = response.read().decode("utf-8")
            html = html.replace("<head>", '<head><base href="http://iris.test/">', 1)
            page.set_content(html, wait_until="networkidle")
            # set_content() lives on about:blank, so <base> would otherwise turn a
            # fragment-only anchor into a document navigation. Mirror the same-document
            # fragment behavior of the real loopback origin while still exercising click.
            page.evaluate("""() => document.addEventListener('click', event => {
              const link = event.target.closest?.('a[href^="#"]');
              if (!link) return;
              event.preventDefault();
              location.hash = link.getAttribute('href').slice(1);
            }, true)""")
            page.wait_for_selector("#globalSearch")

            assert page.locator(".home-resources", has_text="Ресурсы Iris Online").count() == 1
            assert page.locator('.home-resources a[href="https://irisonline.ru/"]').count() == 1
            assert page.locator('.home-resources a[href="https://wiki.irisonline.ru/"]').count() == 1
            assert page.locator('.home-resources a[href="https://vk.com/irisonru"]').count() == 1
            assert page.locator('.home-resources a[href="https://vk.ru/board59626511"]').count() == 1
            assert page.locator(".home-resources", has_text="Официальный статус этих площадок не подтверждён").count() == 1
            assert page.locator(".quick-links", has_text="Быстрый переход").count() == 0
            home_widths = page.evaluate("""() => {
              const primary = document.querySelector('.home-primary').getBoundingClientRect();
              const activity = document.querySelector('.home-activity').getBoundingClientRect();
              return {primaryLeft: primary.left, primaryRight: primary.right, activityLeft: activity.left, activityRight: activity.right};
            }""")
            assert abs(home_widths["primaryLeft"] - home_widths["activityLeft"]) <= 1.5, home_widths
            assert abs(home_widths["primaryRight"] - home_widths["activityRight"]) <= 1.5, home_widths
            server_select = page.locator("#serverSelect")
            server_select.select_option("original")
            assert "Original" in page.locator(".home-database-status").inner_text(), "home server status did not update"
            server_select.select_option("kiss")
            assert "Iris Kiss Kiss" in page.locator(".home-database-status").inner_text(), "home server status did not update back"
            search = page.locator("#globalSearch")
            for query in ("г", "гн", "гнев", "гнев пред", "гнев предков"):
                search.fill(query)
                page.wait_for_timeout(35)
            page.wait_for_timeout(500)
            assert search.evaluate("element => document.activeElement === element"), "search lost focus"
            assert page.locator("#searchSuggestions [role=option]").count() > 0
            search_overlay = page.evaluate("""() => {
              const primary = document.querySelector('.home-primary');
              const suggestions = document.querySelector('#searchSuggestions');
              const primaryRect = primary.getBoundingClientRect();
              const suggestionRect = suggestions.getBoundingClientRect();
              const probeX = Math.min(window.innerWidth - 2, suggestionRect.left + 20);
              const probeY = Math.min(window.innerHeight - 2, suggestionRect.bottom - 4);
              const hit = document.elementFromPoint(probeX, probeY);
              return {
                overflow: getComputedStyle(primary).overflow,
                primaryBottom: primaryRect.bottom,
                suggestionBottom: suggestionRect.bottom,
                hitInsideSuggestions: !!hit && suggestions.contains(hit),
              };
            }""")
            assert search_overlay["overflow"] == "visible", search_overlay
            assert search_overlay["suggestionBottom"] > search_overlay["primaryBottom"] + 8, search_overlay
            assert search_overlay["hitInsideSuggestions"], search_overlay
            search.press("ArrowDown")
            assert search.get_attribute("aria-activedescendant")
            search.press("Escape")
            assert search.get_attribute("aria-expanded") == "false"
            search.fill("")

            page.evaluate("location.hash = 'items'")
            page.wait_for_selector('[data-action="open-filters"]')
            page.locator('[data-action="open-filters"]').click()
            assert page.locator("#filterDrawer").is_visible()
            page.keyboard.press("Escape")
            assert page.locator("#filterDrawer").is_hidden()

            page.locator("#moreButton").click()
            assert page.locator("#moreMenu").is_visible()
            page.keyboard.press("Escape")
            assert page.locator("#moreMenu").is_hidden()
            assert page.locator("#moreButton").evaluate("element => document.activeElement === element")

            page.locator("#moreButton").click()
            page.locator('[data-menu-action="about"]').click()
            assert page.locator("#infoDialog").is_visible()
            about_text = page.locator("#infoDialogBody").inner_text()
            assert "неофициальное фанатское приложение" in about_text
            assert "не связан с разработчиками, издателями или правообладателями" in about_text
            about_link = page.locator('#infoDialogBody a[href="https://irisonline.ru/"]')
            assert about_link.count() == 1
            assert about_link.get_attribute("rel") == "noopener noreferrer"
            page.keyboard.press("Escape")
            assert page.locator("#infoDialog").is_hidden()

            footer = page.locator(".app-footer")
            assert footer.count() == 1
            footer_text = footer.inner_text()
            assert "неофициальное фанатское приложение" in footer_text
            assert "Официальный сайт игры: irisonline.ru" in footer_text
            assert footer.evaluate("element => getComputedStyle(element).position") == "static"

            contrast_script = r"""element => {
              const parse = value => (value.match(/[\d.]+/g) || []).slice(0, 3).map(Number);
              const luminance = rgb => {
                const channels = rgb.map(value => {
                  const normalized = value / 255;
                  return normalized <= 0.03928 ? normalized / 12.92 : Math.pow((normalized + 0.055) / 1.055, 2.4);
                });
                return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
              };
              const foreground = luminance(parse(getComputedStyle(element.querySelector('.app-footer-inner')).color));
              const background = luminance(parse(getComputedStyle(document.body).backgroundColor));
              return (Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05);
            }"""
            for theme in ("dark", "light"):
                page.evaluate("theme => { document.documentElement.dataset.theme = theme; }", theme)
                page.wait_for_timeout(20)
                contrast = footer.evaluate(contrast_script)
                assert contrast >= 4.5, f"footer contrast is too low in {theme} theme: {contrast}"
                select_colors = page.locator("#serverSelect").evaluate(r"""element => {
                  const parse = value => (value.match(/[\d.]+/g) || []).slice(0, 3).map(Number);
                  const luminance = rgb => {
                    const channels = rgb.map(value => {
                      const normalized = value / 255;
                      return normalized <= 0.03928 ? normalized / 12.92 : Math.pow((normalized + 0.055) / 1.055, 2.4);
                    });
                    return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
                  };
                  const style = getComputedStyle(element);
                  const foreground = luminance(parse(style.color));
                  const background = luminance(parse(style.backgroundColor));
                  const option = element.options[0] ? getComputedStyle(element.options[0]) : null;
                  return {
                    ratio: (Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05),
                    optionColor: option?.color || '',
                    optionBackground: option?.backgroundColor || '',
                    border: style.borderColor,
                    radius: style.borderRadius,
                    height: style.minHeight,
                  };
                }""")
                assert select_colors["ratio"] >= 4.5, f"server selector contrast is too low in {theme} theme: {select_colors}"
                assert select_colors["optionColor"] and select_colors["optionBackground"], f"server option colors are missing in {theme} theme"

            for width in (320, 375, 768, 1024, 1440):
                page.set_viewport_size({"width": width, "height": 900})
                page.wait_for_timeout(100)
                overflow = page.evaluate("document.documentElement.scrollWidth > document.documentElement.clientWidth")
                assert not overflow, f"horizontal overflow at {width}px"
                footer_overflow = footer.evaluate("element => element.scrollWidth > element.clientWidth")
                assert not footer_overflow, f"footer overflow at {width}px"

            page.evaluate("location.hash = 'item/80243'")
            page.wait_for_selector(".game-properties")
            item_heading = page.locator(".detail-heading")
            assert item_heading.locator(".rarity-label").count() == 1
            assert item_heading.locator(".class-label", has_text="Боец / Рукопашник").count() == 1
            assert item_heading.locator(".set-label").count() == 1
            properties = page.locator(".game-properties")
            property_text = re.sub(r"\s+", " ", properties.inner_text()).strip()
            for expected in ("Физ. защита: 487", "Маг. защита: 189", "Вес: 98", "Физ. атака: +44", "Выносливость: +47", "Цена продажи: 2,950 тер"):
                assert expected in property_text, f"missing item property: {expected}"
            slot_row = properties.locator(".property-card-slots")
            assert slot_row.count() == 1
            assert slot_row.get_attribute("aria-label") == "Слоты карт: B"
            assert slot_row.locator(".card-slot-chip", has_text="B").count() == 1
            badge_layout = item_heading.locator(".rarity-label, .class-label, .set-label").evaluate_all("elements => elements.map(element => ({display:getComputedStyle(element).display, align:getComputedStyle(element).alignItems, justify:getComputedStyle(element).justifyContent, box:getComputedStyle(element).boxSizing, lineHeight:getComputedStyle(element).lineHeight}))")
            assert all(row["display"] == "flex" and row["align"] == "center" and row["justify"] == "center" and row["box"] == "border-box" for row in badge_layout), badge_layout
            slot_layout = slot_row.locator(".card-slot-chip").evaluate("element => ({display:getComputedStyle(element).display, align:getComputedStyle(element).alignItems, justify:getComputedStyle(element).justifyContent, box:getComputedStyle(element).boxSizing})")
            assert slot_layout == {"display": "flex", "align": "center", "justify": "center", "box": "border-box"}, slot_layout
            assert "Класс:" not in property_text

            # Recently viewed persists compact item/monster records and keeps equal numeric IDs distinct.
            page.evaluate("location.hash = 'item/253'")
            page.wait_for_selector('.detail-page[data-route="item/253"]')
            item_253_name = page.locator('.detail-heading h1').inner_text()
            page.evaluate("location.hash = 'monster/253'")
            page.wait_for_selector('.detail-page[data-route="monster/253"]')
            monster_253_name = page.locator('.detail-heading h1').inner_text()
            page.evaluate("location.hash = 'home'")
            page.wait_for_selector('.recent-viewed-list')
            recent_hrefs = page.locator('.recent-viewed-list a').evaluate_all("links => links.map(link => link.getAttribute('href'))")
            assert recent_hrefs[:2] == ['#monster/253', '#item/253'], recent_hrefs
            assert page.locator('.recent-viewed-list a[href="#monster/253"]', has_text=monster_253_name).count() == 1
            assert page.locator('.recent-viewed-list a[href="#item/253"]', has_text=item_253_name).count() == 1
            page.evaluate("location.hash = 'monster/253'")
            page.wait_for_selector('.detail-page[data-route="monster/253"]')
            page.evaluate("location.hash = 'home'")
            page.wait_for_selector('.recent-viewed-list')
            recent_hrefs = page.locator('.recent-viewed-list a').evaluate_all("links => links.map(link => link.getAttribute('href'))")
            assert recent_hrefs.count('#monster/253') == 1 and recent_hrefs[0] == '#monster/253', recent_hrefs
            page.wait_for_function("""async () => {
              const response = await fetch('/api/user-data');
              const profile = await response.json();
              return Array.isArray(profile.recentlyViewed) && profile.recentlyViewed.some(row => row.type === 'monster' && row.id === 253) && profile.recentlyViewed.some(row => row.type === 'item' && row.id === 253);
            }""", timeout=3000)

            page.evaluate("location.hash = 'item/80243'")
            page.wait_for_selector('.detail-page[data-route="item/80243"]')
            assert page.locator("details summary", has_text="Комплект").count() == 0
            assert properties.locator(".item-inline-set").count() == 1
            page.evaluate("location.hash = 'item/80567'")
            page.wait_for_selector('.detail-page[data-route="item/80567"]')
            set_links = page.locator(".set-member-list .set-member-link").evaluate_all("links => links.map(link => Number(link.getAttribute('href').split('/').pop()))")
            assert set_links == [80567, 80568, 80570, 80569, 80571], f"unexpected equipment order: {set_links}"
            threshold_five = page.locator(".set-effect-threshold", has_text=re.compile(r"^5 предметов"))
            assert threshold_five.count() == 1, "5-piece threshold missing from UI"
            assert "Маг. урон +80" in threshold_five.inner_text(), threshold_five.inner_text()
            assert page.locator("details summary", has_text="Комплект").count() == 0

            left_edge = page.evaluate("""() => {
              const selectors = ['.detail-heading--item h1', '.detail-heading--item p', '.detail-labels', '.game-properties', '.property-card-slots', '.item-inline-set h2', '.set-name', '.set-effects h3'];
              return selectors.map(selector => { const node = document.querySelector(selector); return [selector, node ? node.getBoundingClientRect().left : null]; });
            }""")
            present_edges = [(name, x) for name, x in left_edge if x is not None]
            baseline = present_edges[0][1]
            assert all(abs(x - baseline) <= 1.5 for _, x in present_edges), f"item detail is not left-aligned: {present_edges}"

            fixture_result = page.evaluate("""() => {
              const host = document.createElement('div');
              host.style.cssText = 'position:fixed;left:4px;top:4px;z-index:-1;display:flex;gap:4px';
              const specs = [
                ['rarity-label quality-epic', 'Эпическое'],
                ['meta-label class-label', 'Снайпер / Стрелок'],
                ['meta-label set-label', 'Комплект · 5'],
                ['card-slot-chip', 'A'], ['card-slot-chip', 'B'], ['card-slot-chip', 'AB'], ['card-slot-chip', 'O']
              ];
              for (const [className, text] of specs) { const el=document.createElement('span'); el.className=className; el.textContent=text; host.appendChild(el); }
              document.body.appendChild(host);
              const result = [...host.children].map(el => {
                const range=document.createRange(); range.selectNodeContents(el);
                const outer=el.getBoundingClientRect(); const inner=range.getBoundingClientRect();
                return {text:el.textContent, dx:Math.abs((outer.left+outer.width/2)-(inner.left+inner.width/2)), dy:Math.abs((outer.top+outer.height/2)-(inner.top+inner.height/2)), display:getComputedStyle(el).display, align:getComputedStyle(el).alignItems, justify:getComputedStyle(el).justifyContent};
              });
              host.remove(); return result;
            }""")
            assert all(row['display'] == 'flex' and row['align'] == 'center' and row['justify'] == 'center' and row['dx'] <= 1.5 and row['dy'] <= 2.0 for row in fixture_result), fixture_result

            member_routes = ['80568', '80570', '80569', '80571', '80567']
            warm_nodes = page.evaluate("document.querySelectorAll('*').length")
            page.evaluate("""() => {
              window.__irisFlickerAudit = {stateMessages: 0, detailMissing: 0};
              window.__irisFlickerObserver?.disconnect();
              const host = document.getElementById('mainContent');
              window.__irisFlickerObserver = new MutationObserver(() => {
                if (host.querySelector('.state-message')) window.__irisFlickerAudit.stateMessages += 1;
                if (!host.querySelector('.detail-page')) window.__irisFlickerAudit.detailMissing += 1;
              });
              window.__irisFlickerObserver.observe(host, {childList: true, subtree: true});
            }""")
            for index in range(50):
                target = member_routes[index % len(member_routes)]
                clicked = page.evaluate("target => { const link=document.querySelector(`.set-member-link[href=\"#item/${target}\"]`); if (!link) return false; link.click(); return true; }", target)
                assert clicked, f"set member link not found for {target}"
                page.wait_for_selector(f'.detail-page[data-route="item/{target}"]', timeout=5000)
            final_nodes = page.evaluate("document.querySelectorAll('*').length")
            flicker_audit = page.evaluate("""() => { const result={...window.__irisFlickerAudit}; window.__irisFlickerObserver?.disconnect(); return result; }""")
            assert flicker_audit['stateMessages'] == 0, f"full loading state appeared during set transitions: {flicker_audit}"
            assert flicker_audit['detailMissing'] == 0, f"detail page disappeared during set transitions: {flicker_audit}"
            assert final_nodes <= warm_nodes + 80, f"DOM node count grew across set transitions: {warm_nodes} -> {final_nodes}"

            page.evaluate("""() => {
              window.__irisFlickerAudit = {stateMessages: 0, detailMissing: 0};
              const host = document.getElementById('mainContent');
              window.__irisFlickerObserver = new MutationObserver(() => {
                if (host.querySelector('.state-message')) window.__irisFlickerAudit.stateMessages += 1;
                if (!host.querySelector('.detail-page')) window.__irisFlickerAudit.detailMissing += 1;
              });
              window.__irisFlickerObserver.observe(host, {childList: true, subtree: true});
              location.hash = 'item/80568';
              queueMicrotask(() => { location.hash = 'item/80570'; location.hash = 'item/80569'; });
            }""")
            page.wait_for_selector('.detail-page[data-route="item/80569"]', timeout=5000)
            rapid_audit = page.evaluate("""() => { const result={...window.__irisFlickerAudit}; window.__irisFlickerObserver?.disconnect(); return result; }""")
            assert rapid_audit['stateMessages'] == 0 and rapid_audit['detailMissing'] == 0, f"rapid set navigation flickered: {rapid_audit}"
            assert not js_errors, f"JavaScript errors after aborted requests: {js_errors}"

            # A failed item-to-item request keeps the previous detail page, shows a
            # compact error, restores the visible route, and can be retried immediately.
            page.evaluate("location.hash = 'item/80569'")
            page.wait_for_selector('.detail-page[data-route="item/80569"]', timeout=5000)
            visible_before_failure = page.locator('.detail-page').get_attribute('data-route')
            page.evaluate("""() => {
              const originalFetch = window.fetch;
              window.fetch = (input, init) => {
                if (String(input).startsWith('/api/items/80568?')) {
                  window.fetch = originalFetch;
                  return Promise.resolve(new Response('forced smoke failure', {status: 500, headers: {'Content-Type': 'text/plain'}}));
                }
                return originalFetch(input, init);
              };
              // set_content() has an opaque about:blank origin. In production the
              // app runs at loopback and history.replaceState('#...') is same-origin;
              // capture that call here so the synthetic harness does not throw a
              // SecurityError before the toast can be tested.
              const originalReplaceState = history.replaceState.bind(history);
              window.__irisReplaceStateURL = null;
              history.replaceState = (state, title, url) => {
                window.__irisReplaceStateURL = String(url);
                history.replaceState = originalReplaceState;
              };
            }""")
            failed_clicked = page.evaluate("""() => { const link=document.querySelector('.set-member-link[href="#item/80568"]'); if (!link) return false; link.click(); return true; }""")
            assert failed_clicked
            page.wait_for_timeout(250)
            failure_state = page.evaluate("""() => ({hash: location.hash, route: document.querySelector('.detail-page')?.dataset.route || null, toastHidden: document.querySelector('#toast')?.hidden, toastText: document.querySelector('#toast')?.textContent || '', busy: document.querySelector('.detail-page')?.getAttribute('aria-busy') || null})""")
            assert failure_state['toastHidden'] is False, f"failure toast not visible: {failure_state}"
            assert page.locator('.state-message').count() == 0, "failed item transition replaced detail with full-page error/loading"
            assert page.locator('.detail-page').get_attribute('data-route') == visible_before_failure
            assert page.evaluate("window.__irisReplaceStateURL") == f"#{visible_before_failure}"
            assert "Не удалось открыть предмет" in page.locator('#toast').inner_text()
            page.evaluate("route => { location.hash = route; }", visible_before_failure)
            page.wait_for_selector(f'.detail-page[data-route="{visible_before_failure}"]', timeout=5000)
            retry_clicked = page.evaluate("""() => { const link=document.querySelector('.set-member-link[href="#item/80568"]'); if (!link) return false; link.click(); return true; }""")
            assert retry_clicked
            page.wait_for_selector('.detail-page[data-route="item/80568"]', timeout=5000)

            page.evaluate("location.hash = 'item/253'")
            page.wait_for_selector('.detail-page[data-route="item/253"]')
            page.wait_for_selector(".property-card-slots")
            slot_codes = page.locator(".property-card-slots .card-slot-chip").all_inner_texts()
            assert slot_codes == ["A", "AB"], f"unexpected card slot types: {slot_codes}"
            page.evaluate("location.hash = 'item/80243'")
            page.wait_for_selector('.detail-page[data-route="item/80243"]')
            page.wait_for_selector(".game-properties")
            properties = page.locator(".game-properties")

            assert page.locator("details summary", has_text="Все характеристики").count() == 0
            assert page.locator("details summary", has_text="Дополнительные эффекты").count() == 0
            base = properties.locator(".property-group--base")
            bonus = properties.locator(".property-group--bonus")
            price = properties.locator(".property-group--price")
            assert base.count() == 1 and bonus.count() == 1 and price.count() == 1
            order = page.evaluate("""() => {
              const base = document.querySelector('.property-group--base');
              const bonus = document.querySelector('.property-group--bonus');
              const price = document.querySelector('.property-group--price');
              const source = document.querySelector('.source-overview');
              const before = (left, right) => Boolean(left && right && (left.compareDocumentPosition(right) & Node.DOCUMENT_POSITION_FOLLOWING));
              return { baseBonus: before(base, bonus), bonusPrice: before(bonus, price), priceSource: !source || before(price, source) };
            }""")
            assert all(order.values()), f"incorrect item information order: {order}"
            row_styles = properties.locator(".property-row").evaluate_all("""rows => rows.map(row => ({
              display: getComputedStyle(row).display,
              borderBottom: getComputedStyle(row).borderBottomWidth,
              gridTemplate: getComputedStyle(row).gridTemplateColumns,
              text: row.innerText.trim(),
            }))""")
            assert all(row["display"] == "flex" for row in row_styles), row_styles
            assert all(row["borderBottom"] == "0px" for row in row_styles), row_styles
            assert all(row["gridTemplate"] == "none" for row in row_styles), row_styles
            row_texts = [row["text"] for row in row_styles]
            assert len(row_texts) == len(set(row_texts)), f"duplicate property rows: {row_texts}"
            colors = page.evaluate("""() => ({
              base: getComputedStyle(document.querySelector('.property-row--base .property-value')).color,
              bonus: getComputedStyle(document.querySelector('.property-row--bonus .property-value')).color,
            })""")
            assert colors["base"] != colors["bonus"], colors
            bonus_contrast_script = r"""() => {
              const parse = value => (value.match(/[\d.]+/g) || []).slice(0, 3).map(Number);
              const luminance = rgb => {
                const channels = rgb.map(value => {
                  const normalized = value / 255;
                  return normalized <= 0.03928 ? normalized / 12.92 : Math.pow((normalized + 0.055) / 1.055, 2.4);
                });
                return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
              };
              const foreground = luminance(parse(getComputedStyle(document.querySelector('.property-row--bonus .property-value')).color));
              const background = luminance(parse(getComputedStyle(document.body).backgroundColor));
              return (Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05);
            }"""
            for theme in ("dark", "light"):
                page.evaluate("theme => { document.documentElement.dataset.theme = theme; }", theme)
                assert page.evaluate(bonus_contrast_script) >= 4.5, f"bonus contrast is too low in {theme} theme"
            assert page.locator("details summary", has_text="Описание").count() == 0, "empty item description is visible"
            page.set_viewport_size({"width": 320, "height": 900})
            assert not item_heading.evaluate("element => element.scrollWidth > element.clientWidth"), "item heading overflows at 320px"

            page.evaluate("location.hash = 'items'")
            page.wait_for_selector("[data-catalog-sort]")
            select_classes = page.locator("select").evaluate_all("elements => elements.map(element => element.classList.contains('control-select'))")
            assert all(select_classes), f"not every select uses control-select: {select_classes}"
            shared_styles = page.locator("#serverSelect, [data-catalog-sort]").evaluate_all("""elements => elements.map(element => {
              const style = getComputedStyle(element);
              return [style.backgroundColor, style.color, style.borderColor, style.borderRadius, style.minHeight];
            })""")
            assert shared_styles[0] == shared_styles[1], f"server and sort selects differ: {shared_styles}"

            page.evaluate("location.hash = 'monsters'")
            page.wait_for_selector(".result-row")
            assert page.locator(".result-tertiary", has_text=re.compile(r"\bID\s+\d+")).count() == 0, "monster ID is visible in catalog previews"

            page.evaluate("location.hash = 'monster/141'")
            page.wait_for_selector(".detail-summary")
            assert page.locator(".game-properties").count() == 1
            assert page.locator(".game-properties", has_text="ID монстра").count() == 0
            assert page.locator("details summary", has_text="Все характеристики").count() == 0
            assert page.locator("details summary", has_text="Описание").count() == 0, "empty monster description is visible"
            assert page.locator("details", has_text="ID монстра").count() == 1
            monster_rows = page.locator(".game-properties .property-row")
            assert monster_rows.count() > 0
            assert all(value == "0px" for value in monster_rows.evaluate_all("rows => rows.map(row => getComputedStyle(row).borderBottomWidth)"))

            page.locator("#moreButton").click()
            assert page.locator("#moreMenu", has_text="Пожелания и замечания").count() == 1
            page.keyboard.press("Escape")

            page.evaluate("location.hash = 'monster/108'")
            page.wait_for_selector('.detail-page[data-route="monster/108"]')
            page.wait_for_selector("details.lazy-monster-drops")
            owl_accordion = page.locator("details.lazy-monster-drops")
            owl_accordion.locator(":scope > summary").click()
            page.wait_for_selector("[data-drop-group]")
            owl_group = page.locator('[data-drop-group]', has_text='Группа 10831')
            assert owl_group.count() == 1, "Шипастая сова group 10831 is missing"
            owl_group.locator(":scope > summary").click()
            owl_leggings = owl_group.locator('[data-drop-group-host] > a', has_text='Поножи со следами битв')
            page.wait_for_timeout(30)
            assert owl_leggings.count() == 1, "battle leggings are missing from Шипастая сова"
            owl_drop_text = owl_leggings.inner_text()
            assert '0,0833%' in owl_drop_text, owl_drop_text
            assert '0,0000035%' in owl_drop_text, owl_drop_text
            assert '1 из 28,6 млн' in owl_drop_text, owl_drop_text
            assert '0,0000%' not in owl_drop_text, owl_drop_text

            page.evaluate("location.hash = 'monster/10042'")
            page.wait_for_selector("details.lazy-monster-drops")
            accordion = page.locator("details.lazy-monster-drops")
            assert accordion.count() == 1
            assert page.locator("[data-drop-group-host] > a").count() == 0, "drop items were rendered before opening"
            accordion.locator(":scope > summary").click()
            page.wait_for_selector("[data-drop-group]")
            assert page.locator("[data-drop-group-host] > a").count() == 0, "group items were rendered before group opening"
            group = page.locator("[data-drop-group]").filter(has_text=re.compile(r"[1-9][0-9 ]*\s+(?:предмет|предмета|предметов)" )).first
            group.locator(":scope > summary").click()
            page.wait_for_selector("[data-drop-group-host] > a")
            assert not page.evaluate("document.documentElement.scrollWidth > document.documentElement.clientWidth"), "expanded drop group overflows the mobile viewport"
            initial = page.locator("[data-drop-group-host] > a").count()
            assert 0 < initial <= 30, f"unexpected initial lazy batch: {initial}"
            assert page.locator(".lazy-list-status").count() > 0
            show_all = group.locator("[data-drop-all]")
            if show_all.count():
                show_all.click()
                page.wait_for_timeout(50)
                status = group.locator(".lazy-list-status").inner_text()
                shown, total = [int(value.replace(" ", "")) for value in re.findall(r"\d[\d ]*", status)[:2]]
                assert shown == total, f"show all did not render every row: {status}"

            for _ in range(20):
                accordion.locator(":scope > summary").click()
                accordion.locator(":scope > summary").click()
            for index in range(30):
                route = 'monster/141' if index % 2 else 'monster/10042'
                page.evaluate("route => { location.hash = route; }", route)
                page.wait_for_selector('.detail-page')
            for index in range(20):
                page.locator('#serverSelect').select_option('original' if index % 2 else 'kiss')
                page.wait_for_selector('.detail-page')
            page.evaluate("location.hash = 'items'")
            page.wait_for_selector('[data-action="open-filters"]')
            assert page.locator('[data-drop-group-host] > a').count() == 0, "lazy drop DOM survived route change"

            for route in ("items", "monsters", "favorites", "items", "monster/10042", "items") * 4:
                page.evaluate("route => { location.hash = route; }", route)
                page.wait_for_timeout(80)
            assert not js_errors, f"JavaScript errors: {js_errors}"
            browser.close()

    print("UI smoke test: PASS")


if __name__ == "__main__":
    main()
