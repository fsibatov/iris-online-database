(() => {
  'use strict';

  const APP_VERSION = '1.0.1';
  const PAGE_SIZE = 24;
  const FAVORITES_PAGE_SIZE = 24;
  const SOURCE_BATCH = 20;
  const DROP_BATCH = 30;
  const WORLD_SOURCE_BATCH = 50;
  const SEARCH_DEBOUNCE = 270;
  const REQUEST_TIMEOUT = 15000;
  const PROFILE_DEBOUNCE = 400;
  const VISIBILITY_SAVE_INTERVAL = 10000;
  const PROFILE_PENDING_KEY = 'iris-profile-pending';
  const RECENT_VIEWED_KEY = 'iris-recently-viewed';
  const RECENT_VIEWED_LIMIT = 8;

  function safeJSON(value, fallback) {
    try { return JSON.parse(value); } catch (_) { return fallback; }
  }

  const legacyFavorites = safeJSON(localStorage.getItem('iris-favorites') || '[]', []);
  const state = {
    meta: null,
    effectSpecs: {},
    route: 'home',
    server: localStorage.getItem('iris-server') || 'kiss',
    theme: localStorage.getItem('iris-theme') || 'dark',
    view: localStorage.getItem('iris-view') || 'list',
    favorites: new Set(Array.isArray(legacyFavorites) ? legacyFavorites : []),
    history: [],
    recentlyViewed: safeJSON(localStorage.getItem(RECENT_VIEWED_KEY) || '[]', []),
    profileLoaded: false,
    itemFilters: { q: '', category: '', subcategory: '', quality: '', minLevel: '', maxLevel: '', sort: 'name', page: 1 },
    monsterFilters: { q: '', category: '', type: '', minLevel: '', maxLevel: '', sort: 'name', page: 1 },
    routeController: null,
    catalogController: null,
    suggestionController: null,
    requestId: 0,
    catalog: null,
    sourceSections: [],
    favoritePage: 1,
    monsterDrops: null,
    monsterWorldDrops: null,
    sessionId: '',
  };

  if (!['list', 'cards'].includes(state.view)) state.view = 'list';
  if (!['dark', 'light'].includes(state.theme)) state.theme = 'dark';

  const main = document.getElementById('mainContent');
  const sectionTabs = document.getElementById('sectionTabs');
  const mobileNav = document.getElementById('mobileNav');
  const headerSearchHost = document.getElementById('headerSearchHost');
  const searchWidget = document.getElementById('searchWidget');
  const globalSearch = document.getElementById('globalSearch');
  const suggestions = document.getElementById('searchSuggestions');
  const serverSelect = document.getElementById('serverSelect');
  const moreButton = document.getElementById('moreButton');
  const moreMenu = document.getElementById('moreMenu');
  const themeMenuLabel = document.getElementById('themeMenuLabel');
  const filterDrawer = document.getElementById('filterDrawer');
  const filterDrawerBody = document.getElementById('filterDrawerBody');
  const overlayBackdrop = document.getElementById('overlayBackdrop');
  const resetFiltersButton = document.getElementById('resetFiltersButton');
  const closeFiltersButton = document.getElementById('closeFiltersButton');
  const infoDialog = document.getElementById('infoDialog');
  const infoDialogTitle = document.getElementById('infoDialogTitle');
  const infoDialogBody = document.getElementById('infoDialogBody');
  const toast = document.getElementById('toast');
  const numberFormatter = new Intl.NumberFormat('ru-RU');

  const navItems = [
    { route: 'items', label: 'Предметы', icon: 'item' },
    { route: 'monsters', label: 'Монстры', icon: 'monster' },
    { route: 'favorites', label: 'Избранное', icon: 'star' },
  ];
  const mobileItems = [
    { route: 'home', label: 'Главная', icon: 'home' },
    ...navItems,
  ];

  const icons = {
    home: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m3 11 9-8 9 8"/><path d="M5 10v10h14V10M9 20v-6h6v6"/></svg>',
    item: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 20 8v8l-8 5-8-5V8l8-5Z"/><path d="m4 8 8 5 8-5M12 13v8"/></svg>',
    monster: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 8 4 4M17 8l3-4M6 10c0-3 2.7-5 6-5s6 2 6 5v5c0 3-2.7 5-6 5s-6-2-6-5v-5Z"/><path d="M9 12h.01M15 12h.01M9 16h6"/></svg>',
    star: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-2.9-5.6 2.9 1.1-6.2L3 9.6l6.2-.9L12 3Z"/></svg>',
    search: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6.5"/><path d="m16 16 4 4"/></svg>',
    filter: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16M7 12h10M10 18h4"/></svg>',
    chevron: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 6 6-6 6"/></svg>',
    list: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 6h12M8 12h12M8 18h12M4 6h.01M4 12h.01M4 18h.01"/></svg>',
    grid: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z"/></svg>',
    close: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>',
    info: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 10v6M12 7h.01"/></svg>',
    external: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14 5h5v5M19 5l-8 8"/><path d="M18 13v6H5V6h6"/></svg>',
    arrowLeft: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m15 18-6-6 6-6"/></svg>',
    arrowRight: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 6 6-6 6"/></svg>',
  };

  document.documentElement.dataset.theme = state.theme;
  updateThemeChrome();

  function escapeHTML(value) {
    return String(value ?? '')
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#039;');
  }

  function escapeRegExp(value) {
    return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  function decodeRouteHash() {
    const raw = location.hash.replace(/^#/, '') || 'home';
    try { return decodeURIComponent(raw); } catch (_) { return 'home'; }
  }

  function routeBase(route = state.route) {
    const path = route.split('?')[0];
    if (path.startsWith('item/')) return 'items';
    if (path.startsWith('monster/')) return 'monsters';
    return path;
  }

  function formatNumber(value) {
    return numberFormatter.format(Number(value || 0));
  }

  function russianPlural(value, one, few, many) {
    const number = Math.abs(Math.trunc(Number(value) || 0));
    const lastTwo = number % 100;
    if (lastTwo >= 11 && lastTwo <= 14) return many;
    const last = number % 10;
    if (last === 1) return one;
    if (last >= 2 && last <= 4) return few;
    return many;
  }

  function formatCount(value, one, few, many) {
    return `${formatNumber(value)} ${russianPlural(value, one, few, many)}`;
  }

  function formatSalePrice(value) {
    const number = Math.trunc(Number(value));
    if (!Number.isFinite(number) || number <= 0) return '';
    return `${number.toLocaleString('en-US')} тер`;
  }

  function formatChance(value) {
    const number = Number(value || 0);
    if (!Number.isFinite(number) || number <= 0) return '—';
    const digits = number < 0.0001
      ? Math.min(12, Math.max(8, Math.ceil(-Math.log10(number)) + 1))
      : 4;
    const text = number.toFixed(digits).replace(/0+$/, '').replace(/\.$/, '').replace('.', ',');
    return `${text}%`;
  }

  function formatChanceOdds(value) {
    const chance = Number(value || 0);
    if (!Number.isFinite(chance) || chance <= 0 || chance >= 0.001) return '';
    const oneIn = 100 / chance;
    if (!Number.isFinite(oneIn) || oneIn < 1) return '';
    const formatted = oneIn >= 1_000_000_000
      ? `${(oneIn / 1_000_000_000).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} млрд`
      : oneIn >= 1_000_000
        ? `${(oneIn / 1_000_000).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} млн`
        : Math.round(oneIn).toLocaleString('ru-RU');
    return ` (≈ 1 из ${formatted})`;
  }

  function formatSourceDate(value) {
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(value || ''));
    return match ? `${match[3]}.${match[2]}.${match[1]}` : 'не указана';
  }

  function highlight(value, query) {
    const safe = escapeHTML(value);
    const q = String(query || '').trim();
    if (!q) return safe;
    try { return safe.replace(new RegExp(`(${escapeRegExp(escapeHTML(q))})`, 'ig'), '<mark>$1</mark>'); }
    catch (_) { return safe; }
  }

  function qualityClass(quality) {
    const value = String(quality || '').trim().toLowerCase();
    if (value.includes('уникаль')) return 'quality-unique';
    if (value.includes('эпичес')) return 'quality-epic';
    if (value.includes('редк')) return 'quality-rare';
    if (value.includes('магичес')) return 'quality-magic';
    if (value.includes('покуп')) return 'quality-shop';
    if (value.includes('обыч')) return 'quality-normal';
    return 'quality-default';
  }

  function qualityBadge(quality) {
    const label = String(quality || '').trim();
    if (!label || label.toLocaleLowerCase('ru-RU') === 'не указано') return '';
    return `<span class="rarity-label ${qualityClass(label)}">${escapeHTML(label)}</span>`;
  }

  function itemSetBadge(setSize) {
    const count = Number(setSize || 0);
    return count > 0 ? `<span class="meta-label set-label">Комплект · ${formatNumber(count)}</span>` : '';
  }

  function activeServerMeta() {
    return (state.meta?.servers || []).find(server => normalizeServerKey(server.key) === state.server) || state.meta?.servers?.[0] || {};
  }

  function normalizeServerKey(key) {
    return key === 'or' ? 'original' : key;
  }

  function serverName(server) {
    if (server?.name) return server.name;
    return normalizeServerKey(server?.key) === 'original' ? 'Original' : 'Iris Kiss Kiss';
  }

  function primaryItemStat(item) {
    const stat = (item.stats || [])[0];
    if (!stat) return '';
    const value = stat.name === 'Цена продажи' ? formatSalePrice(stat.value) : stat.value;
    return `${stat.name}: ${value}`;
  }

  function primaryMonsterStat(monster) {
    const stat = (monster.stats || [])[0];
    if (!stat) return '';
    return `${stat.name}: ${formatNumber(stat.value)}`;
  }

  function showToast(message) {
    toast.textContent = message;
    toast.hidden = false;
    clearTimeout(showToast.timer);
    showToast.timer = setTimeout(() => { toast.hidden = true; }, 2200);
  }

  async function api(path, options = {}) {
    const controller = new AbortController();
    const externalSignal = options.signal;
    const abortFromExternal = () => controller.abort(externalSignal?.reason);
    if (externalSignal) {
      if (externalSignal.aborted) controller.abort(externalSignal.reason);
      else externalSignal.addEventListener('abort', abortFromExternal, { once: true });
    }
    const timeout = setTimeout(() => controller.abort(new DOMException('Время ожидания истекло', 'TimeoutError')), REQUEST_TIMEOUT);
    try {
      const headers = { Accept: 'application/json', ...(options.headers || {}) };
      let response;
      try {
        response = await fetch(path, { ...options, headers, signal: controller.signal });
      } catch (error) {
        if (error?.name === 'AbortError' || error?.name === 'TimeoutError') throw error;
        throw new Error('Не удалось связаться с локальным приложением. Попробуйте ещё раз.');
      }
      if (!response.ok) {
        const error = new Error(response.status === 404 ? 'Запись не найдена.' : `Не удалось выполнить запрос (код ${response.status}).`);
        error.status = response.status;
        throw error;
      }
      if (response.status === 204) return null;
      try {
        return await response.json();
      } catch (_) {
        throw new Error('Не удалось прочитать данные приложения.');
      }
    } finally {
      clearTimeout(timeout);
      externalSignal?.removeEventListener('abort', abortFromExternal);
    }
  }

  function loadingPage(label = 'Загрузка данных') {
    main.innerHTML = `<section class="page"><div class="state-message" aria-live="polite"><span class="spinner" aria-hidden="true"></span><h1>${escapeHTML(label)}</h1><p>Подождите немного.</p></div></section>`;
  }

  function errorPage(error) {
    main.innerHTML = `<section class="page"><div class="state-message"><span class="state-symbol" aria-hidden="true">!</span><h1>Не удалось загрузить данные</h1><p>${escapeHTML(error?.message || 'Попробуйте ещё раз.')}</p><button class="primary-button" type="button" data-action="reload">Повторить</button></div></section>`;
  }

  function notFoundPage() {
    main.innerHTML = `<section class="page"><div class="state-message"><span class="state-symbol" aria-hidden="true">404</span><h1>Страница не найдена</h1><p>Проверьте адрес или вернитесь к каталогу.</p><a class="primary-button" href="#items">Открыть предметы</a></div></section>`;
  }

  function pageHeader(title, subtitle = '') {
    return `<header class="page-header"><div><h1>${escapeHTML(title)}</h1>${subtitle ? `<p>${escapeHTML(subtitle)}</p>` : ''}</div></header>`;
  }

  function breadcrumb(parentRoute, parentLabel, current) {
    return `<nav class="breadcrumbs" aria-label="Навигационная цепочка"><a href="#${parentRoute}">${icons.arrowLeft}<span>${escapeHTML(parentLabel)}</span></a><span aria-current="page">${escapeHTML(current)}</span></nav>`;
  }

  function renderNavigation() {
    const active = routeBase();
    sectionTabs.innerHTML = navItems.map(item => `<a href="#${item.route}" class="${active === item.route ? 'active' : ''}" ${active === item.route ? 'aria-current="page"' : ''}>${icons[item.icon]}<span>${item.label}</span></a>`).join('');
    mobileNav.innerHTML = mobileItems.map(item => `<a href="#${item.route}" class="${active === item.route ? 'active' : ''}" ${active === item.route ? 'aria-current="page"' : ''}>${icons[item.icon]}<span>${item.label}</span></a>`).join('');
  }

  function renderServers() {
    if (!state.meta) return;
    const servers = state.meta.servers || [];
    serverSelect.innerHTML = servers.map(server => {
      const key = normalizeServerKey(server.key);
      return `<option value="${escapeHTML(key)}" ${state.server === key ? 'selected' : ''}>${escapeHTML(serverName(server))}</option>`;
    }).join('');
  }

  function positionSearchWidget(home = false) {
    const target = home ? document.getElementById('homeSearchHost') : headerSearchHost;
    if (target && searchWidget.parentElement !== target) target.append(searchWidget);
    searchWidget.classList.toggle('home-search-widget', home);
  }

  function recentViewedEntries() {
    if (!Array.isArray(state.recentlyViewed)) return [];
    return state.recentlyViewed.filter(entry => entry && ['item', 'monster'].includes(entry.type) && Number(entry.id) > 0 && hasMeaningfulText(entry.name)).slice(0, RECENT_VIEWED_LIMIT);
  }

  function trackRecentlyViewed(type, id, name) {
    const numericID = Number(id);
    const cleanName = String(name || '').trim();
    if (!['item', 'monster'].includes(type) || !Number.isInteger(numericID) || numericID <= 0 || !cleanName) return;
    const key = `${type}:${numericID}`;
    const next = [{ type, id: numericID, name: cleanName }, ...recentViewedEntries().filter(entry => `${entry.type}:${entry.id}` !== key)].slice(0, RECENT_VIEWED_LIMIT);
    state.recentlyViewed = next;
    localStorage.setItem(RECENT_VIEWED_KEY, JSON.stringify(next));
    if (state.profileLoaded) scheduleProfileSave(0);
  }

  function clearRecentlyViewed() {
    state.recentlyViewed = [];
    localStorage.setItem(RECENT_VIEWED_KEY, '[]');
    if (state.profileLoaded) scheduleProfileSave(0);
    if (routeBase() === 'home') homePage();
    showToast('Недавно просмотренные очищены.');
  }

  function homePage() {
    const viewed = recentViewedEntries().slice(0, 6);
    const serverLabel = serverName(activeServerMeta());
    const recentlyViewed = viewed.length
      ? `<section class="home-compact-section recently-viewed" aria-labelledby="viewedTitle"><div class="home-section-heading"><h2 id="viewedTitle">Недавно просмотренные</h2><button class="text-button compact-button" type="button" data-action="clear-recently-viewed" aria-label="Очистить недавно просмотренные">Очистить</button></div><div class="recent-viewed-list">${viewed.map(entry => `<a href="#${entry.type}/${entry.id}"><span class="recent-viewed-icon">${icons[entry.type]}</span><span><strong>${escapeHTML(entry.name)}</strong><small>${entry.type === 'item' ? 'Предмет' : 'Монстр'}</small></span>${icons.chevron}</a>`).join('')}</div></section>`
      : `<section class="home-compact-section recently-viewed" aria-labelledby="viewedTitle"><h2 id="viewedTitle">Недавно просмотренные</h2><p class="home-start-hint">Здесь появятся открытые предметы и монстры.</p></section>`;
    const resources = `<section class="home-resources home-compact-section" aria-labelledby="resourcesTitle">
      <h2 id="resourcesTitle">Ресурсы Iris Online</h2>
      <div class="resource-links">
        <a href="https://irisonline.ru/" target="_blank" rel="noopener noreferrer external"><strong>Официальный сайт</strong><small>irisonline.ru</small></a>
        <a href="https://wiki.irisonline.ru/" target="_blank" rel="noopener noreferrer external"><strong>Wiki</strong><small>wiki.irisonline.ru</small></a>
        <a href="https://vk.com/irisonru" target="_blank" rel="noopener noreferrer external"><strong>ВКонтакте</strong><small>vk.com/irisonru</small></a>
        <a href="https://vk.ru/board59626511" target="_blank" rel="noopener noreferrer external"><strong>Обсуждения</strong><small>vk.ru</small></a>
        <a href="https://github.com/fsibatov/iris-online-database" target="_blank" rel="noopener noreferrer external"><strong>GitHub проекта</strong><small>github.com/fsibatov/iris-online-database</small></a>
      </div>
      <div class="community-links" aria-labelledby="communitiesTitle"><h3 id="communitiesTitle">Сообщества</h3><p>Официальный статус этих площадок не подтверждён.</p><div>
        <a href="https://aminoapps.com/c/IrisONru/home/" target="_blank" rel="noopener noreferrer external">Amino</a>
        <a href="https://coub.com/irison.ru" target="_blank" rel="noopener noreferrer external">Coub</a>
        <a href="https://discord.com/invite/m2EPNvV" target="_blank" rel="noopener noreferrer external">Discord</a>
        <a href="https://t.me/joinchat/IZdJ2hPIDvgj-ythC-NvTQ" target="_blank" rel="noopener noreferrer external">Telegram</a>
      </div></div>
    </section>`;
    main.innerHTML = `<section class="page home-page">
      <div class="home-primary">
        <p class="eyebrow">Iris Online</p>
        <h1>Поиск по Iris Online</h1>
        <p>Предметы, монстры, ID и справочные термины.</p>
        <div id="homeSearchHost" class="home-search-host"></div>
      </div>
      <div class="home-activity">${recentlyViewed}${resources}</div>
      <p class="home-database-status">Текущий сервер: <strong data-home-server-name>${escapeHTML(serverLabel)}</strong> · игровые данные загружаются только из локального пакета</p>
    </section>`;
    positionSearchWidget(true);
    requestAnimationFrame(() => globalSearch.focus({ preventScroll: true }));
  }

  function addHistory(query) {
    const value = String(query || '').trim();
    if (!value) return;
    state.history = [value, ...state.history.filter(item => item !== value)].slice(0, 50);
    scheduleProfileSave();
  }

  function submitGlobalSearch() {
    const query = globalSearch.value.trim();
    if (!query) return;
    addHistory(query);
    closeSuggestions();
    location.hash = `search?q=${encodeURIComponent(query)}`;
  }

  let suggestionTimer;
  let suggestionRoutes = [];
  let activeSuggestion = -1;

  function closeSuggestions() {
    suggestions.hidden = true;
    suggestions.innerHTML = '';
    suggestionRoutes = [];
    activeSuggestion = -1;
    globalSearch.setAttribute('aria-expanded', 'false');
    globalSearch.setAttribute('aria-activedescendant', '');
  }

  function setActiveSuggestion(index) {
    const options = [...suggestions.querySelectorAll('[role="option"]')];
    if (!options.length) return;
    activeSuggestion = Math.max(0, Math.min(index, options.length - 1));
    options.forEach((option, position) => option.setAttribute('aria-selected', String(position === activeSuggestion)));
    const active = options[activeSuggestion];
    globalSearch.setAttribute('aria-activedescendant', active.id);
    active.scrollIntoView({ block: 'nearest' });
  }

  function suggestionOption(record, type, query, index) {
    const route = `${type}/${record.id}`;
    const subtitle = type === 'item'
      ? [record.typeLine, record.level ? `Ранг ${record.level}` : '', `ID ${record.id}`].filter(Boolean).join(' · ')
      : [record.category, record.typeName, `Уровень ${record.level}`].filter(Boolean).join(' · ');
    suggestionRoutes.push(route);
    return `<div class="suggestion-option" id="suggestion-${index}" role="option" aria-selected="false" data-suggestion-index="${index}" data-suggestion="${escapeHTML(route)}"><span class="suggestion-type-icon">${icons[type === 'item' ? 'item' : 'monster']}</span><span><strong>${highlight(record.name, query)}</strong><small>${escapeHTML(subtitle)}</small></span></div>`;
  }

  function renderSuggestions(data, query) {
    suggestionRoutes = [];
    let index = 0;
    const groups = [];
    if (data.items?.length) {
      groups.push(`<section class="suggestion-group" aria-label="Предметы"><h2>Предметы</h2>${data.items.map(record => suggestionOption(record, 'item', query, index++)).join('')}</section>`);
    }
    if (data.monsters?.length) {
      groups.push(`<section class="suggestion-group" aria-label="Монстры"><h2>Монстры</h2>${data.monsters.map(record => suggestionOption(record, 'monster', query, index++)).join('')}</section>`);
    }
    suggestions.innerHTML = groups.length ? `${groups.join('')}<button class="suggestion-all" type="button" data-search-all>Показать все результаты для «${escapeHTML(query)}»</button>` : `<div class="suggestion-empty"><strong>Ничего не найдено</strong><span>Попробуйте другое написание или ID.</span></div>`;
    suggestions.hidden = false;
    globalSearch.setAttribute('aria-expanded', 'true');
    activeSuggestion = -1;
    globalSearch.setAttribute('aria-activedescendant', '');
  }

  function updateSuggestions() {
    clearTimeout(suggestionTimer);
    state.suggestionController?.abort();
    const query = globalSearch.value.trim();
    if (query.length < 2) {
      closeSuggestions();
      return;
    }
    suggestionTimer = setTimeout(async () => {
      const controller = new AbortController();
      state.suggestionController = controller;
      try {
        const data = await api(`/api/search?q=${encodeURIComponent(query)}`, { signal: controller.signal });
        if (globalSearch.value.trim() === query) renderSuggestions(data, query);
      } catch (error) {
        if (error?.name !== 'AbortError') closeSuggestions();
      }
    }, SEARCH_DEBOUNCE);
  }

  async function searchPage(query, signal) {
    const params = value => new URLSearchParams({ q: value, page: '1', pageSize: '12', sort: 'name', server: state.server });
    const [itemsData, monstersData] = await Promise.all([
      api(`/api/items?${params(query)}`, { signal }),
      api(`/api/monsters?${params(query)}`, { signal }),
    ]);
    const total = Number(itemsData.total || 0) + Number(monstersData.total || 0);
    main.innerHTML = `<section class="page search-results-page">
      ${pageHeader(`Результаты поиска`, total ? `Результатов по запросу «${query}»: ${formatNumber(total)}.` : `По запросу «${query}» ничего не найдено.`)}
      ${total ? `<div class="search-result-sections">${searchResultSection('Предметы', 'items', itemsData.items || [], itemsData.total, query)}${searchResultSection('Монстры', 'monsters', monstersData.monsters || [], monstersData.total, query)}</div>` : `<div class="state-message compact"><span class="state-symbol">0</span><h2>Нет совпадений</h2><p>Проверьте написание, используйте часть названия или ID.</p></div>`}
    </section>`;
    positionSearchWidget(false);
  }

  function searchResultSection(title, route, records, total, query) {
    if (!records.length) return '';
    const rows = records.map(record => route === 'items' ? itemRow(record, query) : monsterRow(record, query)).join('');
    return `<section class="search-result-section"><header><h2>${title}</h2><a href="#${route}?q=${encodeURIComponent(query)}">Все результаты · ${formatNumber(total)}</a></header><div class="result-list">${rows}</div></section>`;
  }

  function catalogFilters(kind) {
    return kind === 'items' ? state.itemFilters : state.monsterFilters;
  }

  function catalogTitle(kind) {
    return kind === 'items' ? 'Предметы' : 'Монстры';
  }

  function normalizeDependentFilters(kind) {
    const filters = catalogFilters(kind);
    if (!filters.category) {
      if (kind === 'items') {
        filters.subcategory = '';
        filters.quality = '';
      } else filters.type = '';
    }
  }

  function buildCatalogParams(kind) {
    const filters = catalogFilters(kind);
    const params = new URLSearchParams({ page: String(filters.page), pageSize: String(PAGE_SIZE), sort: filters.sort, server: state.server });
    Object.entries(filters).forEach(([key, value]) => {
      if (!['page', 'sort'].includes(key) && value !== '') params.set(key, value);
    });
    return params;
  }

  async function fetchCatalog(kind, signal) {
    return api(`/api/${kind}?${buildCatalogParams(kind)}`, { signal });
  }

  function catalogPage(kind, data) {
    const filters = catalogFilters(kind);
    state.catalog = { kind, data };
    main.innerHTML = `<section class="page catalog-page" data-catalog-kind="${kind}">
      ${pageHeader(catalogTitle(kind), kind === 'items' ? 'Каталог предметов Iris Online.' : 'Каталог монстров Iris Online.')}
      <section class="catalog-controls" aria-label="Управление каталогом">
        <label class="catalog-search"><span class="visually-hidden">Поиск в каталоге</span>${icons.search}<input type="search" data-catalog-search value="${escapeHTML(filters.q)}" placeholder="Поиск в каталоге"></label>
        <button class="secondary-button filter-button" type="button" data-action="open-filters">${icons.filter}<span>Фильтры</span><strong data-filter-count>${activeFilterCount(kind) || ''}</strong></button>
        <label class="sort-control"><span class="visually-hidden">Сортировка</span><select class="control-select" data-catalog-sort aria-label="Сортировка">${sortOptions(kind, filters.sort)}</select></label>
        <div class="view-switch" role="group" aria-label="Вид каталога"><button type="button" data-view="list" class="${state.view === 'list' ? 'active' : ''}" aria-label="Компактный список">${icons.list}</button><button type="button" data-view="cards" class="${state.view === 'cards' ? 'active' : ''}" aria-label="Сетка">${icons.grid}</button></div>
      </section>
      <div class="active-filters" data-active-filters>${activeFilterChips(kind)}</div>
      <div class="catalog-status"><span data-catalog-count>Найдено: ${formatNumber(data.total)}</span><span class="catalog-live" aria-live="polite" data-catalog-live></span></div>
      <div class="catalog-results" data-catalog-results aria-live="polite">${catalogResultsHTML(kind, data)}</div>
      <div data-catalog-pagination>${pagination(data.page, data.pages)}</div>
    </section>`;
    positionSearchWidget(false);
    renderFilterDrawer(kind, data.filters || {});
  }

  function sortOptions(kind, selected) {
    const options = [
      ['name', 'По названию'],
      ['level', kind === 'items' ? 'По рангу' : 'По уровню'],
      ...(kind === 'items' ? [['rarity', 'По редкости']] : []),
    ];
    return options.map(([value, label]) => `<option value="${value}" ${selected === value ? 'selected' : ''}>${label}</option>`).join('');
  }

  function catalogResultsHTML(kind, data) {
    const records = kind === 'items' ? data.items || [] : data.monsters || [];
    if (!records.length) return `<div class="state-message compact"><span class="state-symbol">0</span><h2>Ничего не найдено</h2><p>Измените запрос или сбросьте фильтры.</p><button class="secondary-button" type="button" data-action="reset-filters">Сбросить фильтры</button></div>`;
    return `<div class="result-list ${state.view === 'cards' ? 'card-view' : ''}">${records.map(record => kind === 'items' ? itemRow(record, catalogFilters(kind).q) : monsterRow(record, catalogFilters(kind).q)).join('')}</div>`;
  }

  function itemRow(item, query = '') {
    const key = `item:${item.id}`;
    const active = state.favorites.has(key);
    const secondary = [item.typeLine || item.category, item.level ? `Ранг ${item.level}` : ''].filter(Boolean).join(' · ');
    const tertiary = [qualityBadge(item.quality), primaryItemStat(item) ? `<span>${escapeHTML(primaryItemStat(item))}</span>` : '', itemSetBadge(item.setSize)].filter(Boolean).join('');
    return `<article class="result-row">
      <a class="result-main" href="#item/${item.id}" aria-label="Открыть предмет: ${escapeHTML(item.name)}">
        <span class="result-icon">${icons.item}</span>
        <span class="result-copy"><strong>${highlight(item.name, query)}</strong><span class="result-secondary">${escapeHTML(secondary)}</span><span class="result-tertiary">${tertiary}</span></span>
        <span class="result-arrow">${icons.chevron}</span>
      </a>
      <button class="favorite-button ${active ? 'active' : ''}" type="button" data-favorite="${key}" aria-label="${active ? 'Удалить из избранного' : 'Добавить в избранное'}">${icons.star}</button>
    </article>`;
  }

  function monsterRow(monster, query = '') {
    const key = `monster:${monster.id}`;
    const active = state.favorites.has(key);
    const secondary = [monster.category, monster.typeName, `Уровень ${monster.level}`].filter(Boolean).join(' · ');
    const tertiary = [monster.aggressive ? '<span class="meta-label warning-label">Агрессивный</span>' : '', primaryMonsterStat(monster) ? `<span>${escapeHTML(primaryMonsterStat(monster))}</span>` : ''].filter(Boolean).join('');
    return `<article class="result-row">
      <a class="result-main" href="#monster/${monster.id}" aria-label="Открыть монстра: ${escapeHTML(monster.name)}">
        <span class="result-icon">${icons.monster}</span>
        <span class="result-copy"><strong>${highlight(monster.name, query)}</strong><span class="result-secondary">${escapeHTML(secondary)}</span><span class="result-tertiary">${tertiary}</span></span>
        <span class="result-arrow">${icons.chevron}</span>
      </a>
      <button class="favorite-button ${active ? 'active' : ''}" type="button" data-favorite="${key}" aria-label="${active ? 'Удалить из избранного' : 'Добавить в избранное'}">${icons.star}</button>
    </article>`;
  }

  function pagination(page, pages, action = 'page') {
    if (pages <= 1) return '';
    const buttons = [];
    const start = Math.max(1, Math.min(page - 2, Math.max(1, pages - 4)));
    const end = Math.min(pages, start + 4);
    const attribute = action === 'favorite-page' ? 'data-favorite-page' : 'data-page';
    buttons.push(`<button type="button" ${attribute}="${page - 1}" ${page <= 1 ? 'disabled' : ''} aria-label="Предыдущая страница">${icons.arrowLeft}</button>`);
    for (let i = start; i <= end; i += 1) buttons.push(`<button type="button" ${attribute}="${i}" class="${i === page ? 'active' : ''}" ${i === page ? 'aria-current="page"' : ''}>${i}</button>`);
    buttons.push(`<button type="button" ${attribute}="${page + 1}" ${page >= pages ? 'disabled' : ''} aria-label="Следующая страница">${icons.arrowRight}</button>`);
    return `<nav class="pagination" aria-label="Страницы каталога">${buttons.join('')}</nav>`;
  }

  function activeFilterCount(kind) {
    const filters = catalogFilters(kind);
    const keys = kind === 'items' ? ['category', 'subcategory', 'quality', 'minLevel', 'maxLevel'] : ['category', 'type', 'minLevel', 'maxLevel'];
    return keys.reduce((count, key) => count + (String(filters[key] || '').trim() ? 1 : 0), 0);
  }

  function activeFilterChips(kind) {
    const filters = catalogFilters(kind);
    const labels = kind === 'items'
      ? { category: 'Категория', subcategory: 'Подкатегория', quality: 'Редкость', minLevel: 'Ранг от', maxLevel: 'Ранг до' }
      : { category: 'Категория', type: 'Тип', minLevel: 'Уровень от', maxLevel: 'Уровень до' };
    const chips = Object.entries(labels).filter(([key]) => String(filters[key] || '').trim()).map(([key, label]) => `<button type="button" class="filter-chip" data-clear-filter="${key}" aria-label="Убрать фильтр: ${escapeHTML(label)} ${escapeHTML(filters[key])}"><span>${escapeHTML(key.startsWith('min') || key.startsWith('max') ? `${label} ${filters[key]}` : filters[key])}</span>${icons.close}</button>`);
    if (!chips.length) return '';
    return `${chips.join('')}<button class="clear-all" type="button" data-action="reset-filters">Сбросить всё</button>`;
  }

  function optionList(options, selected, anyLabel) {
    return `<option value="">${escapeHTML(anyLabel)}</option>${(options || []).map(value => `<option value="${escapeHTML(value)}" ${value === selected ? 'selected' : ''}>${escapeHTML(value)}</option>`).join('')}`;
  }

  function renderFilterDrawer(kind, filterData) {
    const filters = catalogFilters(kind);
    const dependentLocked = !filters.category;
    filterDrawer.dataset.kind = kind;
    filterDrawerBody.innerHTML = `<div class="drawer-fields">
      <label class="field"><span>Категория</span><select class="control-select" name="category">${optionList(filterData.categories, filters.category, 'Любая')}</select></label>
      ${kind === 'items'
        ? `<label class="field"><span>Подкатегория</span><select class="control-select" name="subcategory" ${dependentLocked ? 'disabled' : ''}>${optionList(filterData.subcategories, filters.subcategory, 'Любая')}</select>${dependentLocked ? '<small>Сначала выберите категорию.</small>' : ''}</label><label class="field"><span>Редкость</span><select class="control-select" name="quality" ${dependentLocked ? 'disabled' : ''}>${optionList(filterData.qualities, filters.quality, 'Любая')}</select>${dependentLocked ? '<small>Сначала выберите категорию.</small>' : ''}</label>`
        : `<label class="field"><span>Тип монстра</span><select class="control-select" name="type" ${dependentLocked ? 'disabled' : ''}>${optionList(filterData.types, filters.type, 'Любой')}</select>${dependentLocked ? '<small>Сначала выберите категорию.</small>' : ''}</label>`}
      <div class="range-fields"><label class="field"><span>${kind === 'items' ? 'Ранг от' : 'Уровень от'}</span><input name="minLevel" type="number" inputmode="numeric" min="0" value="${escapeHTML(filters.minLevel)}"></label><label class="field"><span>${kind === 'items' ? 'Ранг до' : 'Уровень до'}</span><input name="maxLevel" type="number" inputmode="numeric" min="0" value="${escapeHTML(filters.maxLevel)}"></label></div>
    </div>`;
  }

  let filterReturnFocus = null;
  let dialogReturnFocus = null;
  function setBackgroundInert(inert) {
    [document.querySelector('.topbar'), main, document.querySelector('.mobile-nav')].forEach(element => {
      if (!element) return;
      if (inert) element.setAttribute('inert', '');
      else element.removeAttribute('inert');
    });
  }

  function openFilters() {
    if (!state.catalog) return;
    filterReturnFocus = document.activeElement;
    filterDrawer.hidden = false;
    overlayBackdrop.hidden = false;
    document.body.classList.add('overlay-open');
    setBackgroundInert(true);
    requestAnimationFrame(() => filterDrawer.querySelector('select:not(:disabled), input, button')?.focus());
  }

  function closeFilters() {
    if (filterDrawer.hidden) return;
    filterDrawer.hidden = true;
    overlayBackdrop.hidden = true;
    document.body.classList.remove('overlay-open');
    setBackgroundInert(false);
    filterReturnFocus?.focus?.();
    filterReturnFocus = null;
  }

  let catalogDebounce;
  async function refreshCatalog({ refreshFilters = false, announce = true } = {}) {
    const catalog = state.catalog;
    if (!catalog) return;
    state.catalogController?.abort();
    const controller = new AbortController();
    state.catalogController = controller;
    const results = main.querySelector('[data-catalog-results]');
    const live = main.querySelector('[data-catalog-live]');
    results?.setAttribute('aria-busy', 'true');
    if (live && announce) live.textContent = 'Обновление…';
    try {
      const data = await fetchCatalog(catalog.kind, controller.signal);
      if (controller.signal.aborted || state.catalog?.kind !== catalog.kind) return;
      state.catalog.data = data;
      if (results) results.innerHTML = catalogResultsHTML(catalog.kind, data);
      const count = main.querySelector('[data-catalog-count]');
      if (count) count.textContent = `Найдено: ${formatNumber(data.total)}`;
      const paging = main.querySelector('[data-catalog-pagination]');
      if (paging) paging.innerHTML = pagination(data.page, data.pages);
      const chips = main.querySelector('[data-active-filters]');
      if (chips) chips.innerHTML = activeFilterChips(catalog.kind);
      const filterCount = main.querySelector('[data-filter-count]');
      if (filterCount) filterCount.textContent = activeFilterCount(catalog.kind) || '';
      if (refreshFilters) renderFilterDrawer(catalog.kind, data.filters || {});
      if (live && announce) live.textContent = `Показано ${formatNumber((catalog.kind === 'items' ? data.items : data.monsters).length)} из ${formatNumber(data.total)}.`;
      scheduleProfileSave();
    } catch (error) {
      if (error?.name === 'AbortError') return;
      if (live) live.textContent = 'Не удалось обновить каталог.';
      showToast('Не удалось обновить каталог');
    } finally {
      results?.removeAttribute('aria-busy');
    }
  }

  function resetFilters(kind = state.catalog?.kind || routeBase()) {
    if (kind === 'monsters') state.monsterFilters = { q: '', category: '', type: '', minLevel: '', maxLevel: '', sort: 'name', page: 1 };
    else state.itemFilters = { q: '', category: '', subcategory: '', quality: '', minLevel: '', maxLevel: '', sort: 'name', page: 1 };
    const search = main.querySelector('[data-catalog-search]');
    if (search) search.value = '';
    refreshCatalog({ refreshFilters: true });
  }

  function toggleFavorite(key, button) {
    const added = !state.favorites.has(key);
    if (added) state.favorites.add(key); else state.favorites.delete(key);
    persistFavorites();
    if (button) {
      button.classList.toggle('active', added);
      button.setAttribute('aria-label', added ? 'Удалить из избранного' : 'Добавить в избранное');
    }
    showToast(added ? 'Добавлено в избранное' : 'Удалено из избранного');
    if (routeBase() === 'favorites') renderRoute();
  }

  function compactRange(minimum, maximum) {
    const min = Number(minimum || 0);
    const max = Number(maximum || 0);
    if (!min && !max) return '';
    if (!min || min === max) return formatNumber(max || min);
    if (!max) return formatNumber(min);
    return `${formatNumber(min)}–${formatNumber(max)}`;
  }

  function hasMeaningfulText(value) {
    if (value === null || value === undefined) return false;
    const text = String(value).trim();
    if (!text) return false;
    return !['не указано', 'none', 'null', 'undefined', 'n/a'].includes(text.toLocaleLowerCase('ru-RU'));
  }

  function hasPositiveStat(value) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0;
  }

  function hasGameValue(value, { allowZero = false } = {}) {
    if (value === null || value === undefined) return false;
    const text = String(value).trim();
    if (!text || /^[-—]+$/.test(text)) return false;
    if (allowZero) return true;
    return !/^[+\-−]?0(?:[.,]0+)?%?$/.test(text);
  }

  function usefulPropertyValue(value) {
    return hasGameValue(value);
  }

  function uniquePropertyRows(rows, { allowZero = false } = {}) {
    const seen = new Set();
    return (rows || []).filter(([label, value]) => {
      const cleanLabel = String(label || '').trim();
      const cleanValue = String(value ?? '').trim();
      if (!cleanLabel || !hasGameValue(cleanValue, { allowZero })) return false;
      const key = `${cleanLabel.toLocaleLowerCase('ru-RU')}\u0000${cleanValue.toLocaleLowerCase('ru-RU')}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    }).map(([label, value]) => [String(label).trim(), String(value).trim()]);
  }

  function uniqueTextValues(values) {
    const seen = new Set();
    return (values || []).map(value => String(value ?? '').trim()).filter(value => {
      if (!hasMeaningfulText(value)) return false;
      const key = value.toLocaleLowerCase('ru-RU');
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }

  function itemLevelSummary(item) {
    const minimum = Number(item.minLevel || 0);
    const maximum = Number(item.maxLevel || 0);
    if (minimum > 1 && maximum > 0 && maximum < 100 && maximum !== minimum) return `Ранг ${minimum}–${maximum}`;
    if (minimum > 1) return `Ранг ${minimum}`;
    if (maximum > 0 && maximum < 100) return `Ранг до ${maximum}`;
    return '';
  }

  function isEquipmentItem(item) {
    return [1, 2, 3, 10].includes(Number(item.mainCategoryId));
  }

  const MAKE_SKILL_NAMES = Object.freeze({ 1: 'Кулинар', 2: 'Каллиграф', 3: 'Алхимик', 4: 'Ремесленник', 5: 'Кузнец' });
  const USE_MAP_NAMES = Object.freeze({
    1: 'Только для состязаний',
    2: 'Только для пролога',
    3: 'Везде, включая город',
    4: 'Везде, кроме города',
    5: 'Не для состязаний',
    6: 'Не для подземелий',
    7: 'Только для подземелий',
  });
  const GUILD_USE_NAMES = Object.freeze({ 1: 'Только для главы гильдии', 2: 'Для участников гильдии' });

  function formatDurationSeconds(value) {
    let seconds = Number(value);
    if (!Number.isFinite(seconds) || seconds <= 0) return '';
    if (seconds % 86400 === 0) return `${formatNumber(seconds / 86400)} дн.`;
    if (seconds % 3600 === 0) return `${formatNumber(seconds / 3600)} ч.`;
    if (seconds % 60 === 0) return `${formatNumber(seconds / 60)} мин.`;
    return `${formatNumber(seconds)} с.`;
  }

  function formatDurationMilliseconds(value) {
    const milliseconds = Number(value);
    if (!Number.isFinite(milliseconds) || milliseconds <= 0) return '';
    if (milliseconds % 1000 === 0) return formatDurationSeconds(milliseconds / 1000);
    return `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(milliseconds / 1000)} с.`;
  }

  function formatAttackRange(value) {
    const raw = Number(value);
    if (!Number.isFinite(raw) || raw <= 0) return '';
    return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(raw * 0.01);
  }

  function itemPresentation(item, bonuses) {
    const classes = uniqueTextValues([item.job1Name, item.job2Name]);
    const baseStats = uniquePropertyRows([
      ['Физ. атака', compactRange(item.physicalMin, item.physicalMax)],
      ['Маг. атака', compactRange(item.magicMin, item.magicMax)],
      ['Физ. защита', hasPositiveStat(item.physicalDefense) ? formatNumber(item.physicalDefense) : ''],
      ['Маг. защита', hasPositiveStat(item.magicDefense) ? formatNumber(item.magicDefense) : ''],
      ['Лечение', hasPositiveStat(item.heal) ? formatNumber(item.heal) : ''],
      ['Скорость атаки', hasPositiveStat(item.attackSpeed) ? formatNumber(item.attackSpeed) : ''],
      ['Дальность атаки', formatAttackRange(item.attackRange)],
      ['Перезарядка', formatDurationMilliseconds(item.cooldown)],
      ['Длительность эффекта', formatDurationMilliseconds(item.effectDurationMs)],
    ]);
    const miscStats = uniquePropertyRows([
      ['Вес', hasPositiveStat(item.weight) ? formatNumber(item.weight) : ''],
      ['Максимум в стопке', Number(item.maxStack) > 1 ? formatNumber(item.maxStack) : ''],
    ]);
    // An explicit option row is data even when its numeric value is zero. The
    // source row, not JavaScript truthiness, decides whether the effect exists.
    const knownBonusRows = (bonuses || []).filter(row => Array.isArray(row) || row?.known !== false);
    const unknownBonusRows = (bonuses || []).filter(row => !Array.isArray(row) && row?.known === false);
    const bonusStats = uniquePropertyRows(knownBonusRows.map(row => [row.name ?? row[0], row.value ?? row[1]]), { allowZero: true });
    const unknownStats = uniquePropertyRows(unknownBonusRows.map(row => [row.name, row.value]), { allowZero: true });
    const bonusTexts = uniqueTextValues([item.abilityDescription]);
    const profession = Number(item.makeSkill) > 0
      ? `${MAKE_SKILL_NAMES[Number(item.makeSkill)] || `Код ${item.makeSkill}`} (${formatNumber(Number(item.makeSkillExp) || 0)})`
      : '';
    const useMap = Number(item.useMapType) > 0
      ? USE_MAP_NAMES[Number(item.useMapType)] || `Неизвестный тип ограничения (код ${item.useMapType})`
      : '';
    const guildUse = Number(item.guildUse) > 0
      ? GUILD_USE_NAMES[Number(item.guildUse)] || `Неизвестное ограничение гильдии (код ${item.guildUse})`
      : '';
    const restrictions = uniquePropertyRows([
      ['Раса', item.raceName],
      ['Пол', item.genderName],
      ['Профессия', profession],
      ['Место использования', useMap],
      ['Гильдия', guildUse],
      ['Максимум в инвентаре', Number(item.maxInventory) > 0 ? formatNumber(item.maxInventory) : ''],
      ['Срок действия', Number(item.termSet) === 1 ? formatDurationSeconds(item.termDuration) : ''],
    ]);
    const actions = [];
    if (Number(item.degradationIndex) > 0) actions.push({ text: 'Можно разобрать', tone: 'allowed' });
    if (Number(item.enhancedIndex) > 0) actions.push({ text: 'Можно перековать', tone: 'allowed' });
    if (Number(item.printableFlag) > 0) actions.push({ text: 'Печать доступна', tone: 'allowed' });
    if (Number(item.sellType) === 0) actions.push({ text: 'Нельзя продать в магазин', tone: 'denied' });
    if (isEquipmentItem(item)) {
      actions.push({ text: Number(item.seal) > 0 ? 'Можно запечатать' : 'Запечатать невозможно', tone: Number(item.seal) > 0 ? 'allowed' : 'denied' });
    }
    const price = Number(item.sellType) === 1 && Number(item.price) > 0 ? ['Цена продажи', formatSalePrice(item.price)] : null;
    const cardSlots = (item.cardSlots || []).map(slot => String(slot ?? '').trim()).filter(hasMeaningfulText);
    return { classes, baseStats, bonusStats, unknownStats, bonusTexts, miscStats, restrictions, actions, cardSlots, price };
  }

  function monsterPresentation(monster) {
    return {
      baseStats: uniquePropertyRows([
        ['Здоровье', hasPositiveStat(monster.hp) ? formatNumber(monster.hp) : ''],
        ['Мана', hasPositiveStat(monster.mp) ? formatNumber(monster.mp) : ''],
        ['Физ. защита', hasPositiveStat(monster.defense) ? formatNumber(monster.defense) : ''],
        ['Маг. защита', hasPositiveStat(monster.magicDefense) ? formatNumber(monster.magicDefense) : ''],
        ['Меткость', hasPositiveStat(monster.hit) ? formatNumber(monster.hit) : ''],
        ['Уклонение', hasPositiveStat(monster.evasion) ? formatNumber(monster.evasion) : ''],
        ['Критическая защита', hasPositiveStat(monster.criticalDefense) ? formatNumber(monster.criticalDefense) : ''],
        ['Радиус обзора', hasPositiveStat(monster.viewRange) ? formatNumber(monster.viewRange) : ''],
        ['Радиус атаки', hasPositiveStat(monster.attackRadius) ? formatNumber(monster.attackRadius) : ''],
        ['Скорость ходьбы', hasPositiveStat(monster.walkSpeed) ? formatNumber(monster.walkSpeed) : ''],
        ['Скорость бега', hasPositiveStat(monster.runSpeed) ? formatNumber(monster.runSpeed) : ''],
        ['Дальность преследования', hasPositiveStat(monster.followRange) ? formatNumber(monster.followRange) : ''],
      ]),
      bonusStats: [],
      restrictions: [],
      actions: [],
      price: null,
    };
  }

  function itemTechnicalRows(item) {
    return uniquePropertyRows([
      ['ID предмета', formatNumber(item.id)],
      ['Название — индекс', item.nameIndex],
      ['Описание — индекс', item.tooltipIndex],
      ['Характеристики — индекс', item.abilityIndex],
      ['Карты — индекс', item.cardIndex],
      ['Основная категория', item.mainCategory],
      ['Основная категория — код', item.mainCategoryId],
      ['Средняя категория', item.middleCategory],
      ['Средняя категория — код', item.middleCategoryId],
      ['Подкатегория исходной классификации', item.subCategory],
      ['Подкатегория — код', item.subCategoryId],
      ['Категория каталога', item.category],
      ['Подкатегория каталога', item.subcategory],
      ['Редкость — код', item.qualityId],
      ['Вместимость — исходное значение', item.capacity],
      ['Тип продажи — код', item.sellType],
      ['Передача — код', item.exchange],
      ['Запечатывание — код', item.seal],
      ['Комплект — ID', item.setIndex],
      ['Значок — ID', item.iconIndex],
      ['Раса — код', item.race],
      ['Пол — код', item.gender],
      ['Класс 1 — код', item.job1],
      ['Класс 2 — код', item.job2],
      ['Мин. ранг — исходное значение', item.minLevel],
      ['Макс. ранг — исходное значение', item.maxLevel],
      ['Описание эффекта — индекс', item.abilityDescriptionIndex],
      ['Тип защиты — код', item.defenseType],
      ['Тип дальности — код', item.rangeType],
      ['Тип цели — код', item.targetType],
      ['Дальность использования — исходное значение', item.useRange],
      ['Группа перезарядки — исходное значение', item.groupTime],
      ['Влияние — индекс', item.influenceIndex],
      ['Активный эффект — индекс', item.activeIndex],
      ['Длительность эффекта — мс', item.effectDurationMs],
      ['Место использования — код', item.useMapType],
      ['Профессия — код', item.makeSkill],
      ['Опыт профессии — исходное значение', item.makeSkillExp],
      ['Гильдия — код ограничения', item.guildUse],
      ['Доп. ограничение — тип', item.limitMapTypeRaw],
      ['Доп. ограничение — значение', item.limitValueRaw],
      ['Вид — исходный код', item.kindOf],
      ['Событие — исходный код', item.eventType],
      ['Цена покупки — исходное значение', item.buyPrice],
      ['Валюта покупки — код', item.buyCurrency],
      ['Срок — тип', item.termSet],
      ['Срок — секунд', item.termDuration],
      ['Печать — исходный флаг', item.printableFlag],
      ['Разбор — индекс', item.degradationIndex],
      ['Усиление — индекс', item.enhancedIndex],
      ['Закалка — индекс', item.reinforcingIndex],
      ['Изменение — индекс', item.changeIndex],
      ['Титул — индекс', item.titleIndex],
    ], { allowZero: true });
  }

  function monsterTechnicalRows(monster) {
    return uniquePropertyRows([
      ['ID монстра', formatNumber(monster.id)],
      ['Категория — код', monster.categoryId],
      ['Тип — код', monster.type],
      ['Вид — код', monster.kind],
      ['Класс — код', monster.jobId],
      ['Важность — исходное значение', monster.importance],
      ['Масштаб — исходное значение', monster.scale],
      ['Опыт — исходное значение', monster.exp],
      ['Денежный бонус — исходное значение', monster.moneyBonus],
      ['Восстановление — исходное значение', monster.recovery],
      ['Название — индекс', monster.nameIndex],
      ['Реплика/описание — индекс', monster.noteIndex],
      ['Высота имени — исходное значение', monster.nameHeight],
      ['Служебный флаг источника — код', monster.sourceFlag],
      ['Масштаб эффекта — исходное значение', monster.effectScale],
      ['Радиус свободного перемещения — исходное значение', monster.freeMoveRange],
      ['Доля остановки — исходное значение', monster.actionStopRatio],
      ['Доля ходьбы — исходное значение', monster.actionWalkRatio],
      ['Доля бега — исходное значение', monster.actionRunRatio],
      ['Время остановки — исходное значение', monster.actionStopTime],
      ['Смена монстра — исходный флаг', monster.changeMonsterCheck],
      ['Время преследования — исходное значение', monster.followTime],
      ['Побег — тип', monster.escapeType],
      ['Побег — исходная вероятность', monster.escapePercent],
      ['Восстановление — время', monster.recoveryTime],
    ], { allowZero: true });
  }

  function itemClassBadge(classes) {
    const label = uniqueTextValues(classes).join(' / ');
    return label ? `<span class="meta-label class-label">${escapeHTML(label)}</span>` : '';
  }

  function propertyRows(rows, modifier) {
    if (!rows?.length) return '';
    return rows.map(([label, value]) => {
      const rowModifier = modifier === 'bonus' && /^[-−]/.test(String(value).trim()) ? 'penalty' : modifier;
      return `<div class="property-row property-row--${rowModifier}"><span class="property-name">${escapeHTML(label)}:</span><strong class="property-value">${escapeHTML(value)}</strong></div>`;
    }).join('');
  }

  function cardSlotsRow(slots) {
    const values = (slots || []).map(slot => String(slot || '').trim()).filter(Boolean);
    if (!values.length) return '';
    const accessible = `Слоты карт: ${values.join(', ')}`;
    const chips = values.map(slot => `<span class="card-slot-chip" title="Тип слота: ${escapeHTML(slot)}">${escapeHTML(slot)}</span>`).join('');
    return `<div class="property-card-slots" aria-label="${escapeHTML(accessible)}"><span class="property-name">Слоты карт:</span><span class="card-slot-list" aria-hidden="true">${chips}</span></div>`;
  }

  function gameProperties(presentation, label, inlineSet = '') {
    const groups = [];
    const baseContent = propertyRows(presentation.baseStats, 'base');
    if (baseContent) groups.push(`<div class="property-group property-group--base" aria-label="Основные характеристики">${baseContent}</div>`);
    const bonusContent = `${propertyRows(presentation.bonusStats, 'bonus')}${(presentation.bonusTexts || []).map(text => `<div class="property-text property-text--bonus">${escapeHTML(text).replaceAll('\n', '<br>')}</div>`).join('')}`;
    if (bonusContent) groups.push(`<div class="property-group property-group--bonus" aria-label="Дополнительные эффекты">${bonusContent}</div>`);
    if (presentation.unknownStats?.length) groups.push(`<div class="property-group property-group--unknown" aria-label="Неизвестные игровые параметры">${propertyRows(presentation.unknownStats, 'unknown')}</div>`);
    const slots = cardSlotsRow(presentation.cardSlots);
    if (slots) groups.push(`<div class="property-group property-group--slots" aria-label="Слоты карт">${slots}</div>`);
    if (inlineSet) groups.push(inlineSet);
    if (presentation.miscStats?.length) groups.push(`<div class="property-group property-group--misc" aria-label="Прочие свойства">${propertyRows(presentation.miscStats, 'base')}</div>`);
    if (presentation.restrictions?.length) groups.push(`<div class="property-group property-group--restrictions" aria-label="Требования">${propertyRows(presentation.restrictions, 'restriction')}</div>`);
    if (presentation.actions?.length) groups.push(`<div class="property-group property-group--actions" aria-label="Разрешённые и запрещённые действия">${presentation.actions.map(action => `<div class="property-action property-action--${escapeHTML(action.tone)}">${escapeHTML(action.text)}</div>`).join('')}</div>`);
    if (presentation.price && usefulPropertyValue(presentation.price[1])) groups.push(`<div class="property-group property-group--price" aria-label="Цена продажи">${propertyRows([presentation.price], 'price')}</div>`);
    return groups.length ? `<section class="game-properties" aria-label="${escapeHTML(label)}">${groups.join('')}</section>` : '';
  }

  function recipeMaterialsHTML(materials) {
    if (!Array.isArray(materials) || !materials.length) return '';
    const rows = materials.map(material => {
      const quantity = Math.max(1, Number(material.quantity) || 1);
      const label = hasMeaningfulText(material.item) ? material.item : `Предмет ID ${material.itemId}`;
      return `<a href="#item/${Number(material.itemId)}"><span>${icons.item}</span><strong>${escapeHTML(label)}</strong><small>×${formatNumber(quantity)}</small></a>`;
    }).join('');
    return `<section class="recipe-materials" aria-labelledby="recipeMaterialsTitle"><h2 id="recipeMaterialsTitle">Материалы рецепта</h2><div class="recipe-material-list">${rows}</div></section>`;
  }

  function chestVariantLabel(variant) {
    const details = [];
    const quantity = Number(variant?.quantity) || 0;
    const enhanced = Number(variant?.enhanced) || 0;
    if (quantity > 1) details.push(`×${formatNumber(quantity)}`);
    if (enhanced > 0) details.push(`усиление +${formatNumber(enhanced)}`);
    return details.join(' · ');
  }

  function chestContentsHTML(chest) {
    const items = Array.isArray(chest?.items) ? chest.items : [];
    if (!items.length) return '';
    const rows = items.map(item => {
      const variants = Array.isArray(item.variants) ? item.variants : [];
      let detail = '';
      if (variants.length === 1) {
        detail = chestVariantLabel(variants[0]);
      } else if (variants.length > 1) {
        detail = variants.map(variant => {
          const label = chestVariantLabel(variant) || 'обычный вариант';
          return variant.chanceKnown === true ? `${label} — ${formatChance(variant.chance)}` : label;
        }).join(' · ');
      }
      const body = `<span class="source-icon">${icons.item}</span><span><strong>${escapeHTML(item.item || `Предмет ID ${item.itemId}`)}</strong>${detail ? `<small>${escapeHTML(detail)}</small>` : ''}</span>${item.chanceKnown === true ? `<span class="source-chance"><small>Шанс при открытии</small><strong>${formatChance(item.chance)}</strong></span>` : ''}`;
      return item.itemKnown === true ? `<a class="chest-content-row" href="#item/${Number(item.itemId)}">${body}</a>` : `<div class="chest-content-row">${body}</div>`;
    }).join('');
    const drawCount = Number(chest.drawCount) || 0;
    const drawNote = drawCount > 0 ? `<p>За одно открытие из выбранной группы выбирается ${formatCount(drawCount, 'предмет', 'предмета', 'предметов')}.</p>` : '';
    return `<section class="chest-contents" aria-labelledby="chestContentsTitle"><header><div><span class="eyebrow">Сундук</span><h2 id="chestContentsTitle">Содержимое сундука</h2>${drawNote}</div><button class="text-button" type="button" data-dialog="chance">${icons.info}<span>Как рассчитывается шанс</span></button></header><div class="chest-content-list">${rows}</div></section>`;
  }

  function meaningfulDescription(value, recordName = '') {
    const text = String(value || '').replace(/\r\n?/g, '\n').trim();
    const normalized = text.replace(/\s+/g, ' ').toLocaleLowerCase('ru-RU');
    const normalizedName = String(recordName || '').trim().replace(/\s+/g, ' ').toLocaleLowerCase('ru-RU');
    const placeholder = normalized.replace(/[.!?…]+$/g, '').trim();
    if (!normalized || normalized === normalizedName) return '';
    if (['описание отсутствует', 'нет описания', 'не указано', 'отсутствует', 'нет', 'none', 'null', 'undefined', 'n/a', '0', '-', '—'].includes(placeholder)) return '';
    if (!/[0-9a-zа-яё]/i.test(normalized)) return '';
    return text;
  }

  function kvList(rows) {
    if (!rows?.length) return '<p class="empty-copy">Нет данных.</p>';
    return `<dl class="kv-list">${rows.map(([label, value]) => `<div><dt>${escapeHTML(label)}</dt><dd>${escapeHTML(value)}</dd></div>`).join('')}</dl>`;
  }

  function accordion(title, summary, content, open = false, className = '') {
    return `<details class="detail-accordion ${className}" ${open ? 'open' : ''}><summary><span><strong>${escapeHTML(title)}</strong>${summary ? `<small>${escapeHTML(summary)}</small>` : ''}</span>${icons.chevron}</summary><div class="accordion-content">${content}</div></details>`;
  }

  function itemDetail(data) {
    const item = data.item;
    trackRecentlyViewed('item', item.id, item.name);
    const key = `item:${item.id}`;
    const active = state.favorites.has(key);
    const setMembers = data.set?.items || [];
    const bonuses = Array.isArray(data.bonuses) ? data.bonuses : [];
    const presentation = itemPresentation(item, bonuses);
    const description = meaningfulDescription(item.tooltip, item.name);
    const drops = [...(data.drops || [])];
    const knownDrops = drops.filter(drop => drop.source !== 'Сундук' || drop.chanceKnown === true);
    const best = (knownDrops.length ? knownDrops : drops).reduce((current, row) => !current || baseAttemptChance(row) > baseAttemptChance(current) ? row : current, null);
    const bestChanceKnown = best && (best.source !== 'Сундук' || best.chanceKnown === true);
    const bestLabel = bestChanceKnown ? (best.source === 'Сундук' ? `${formatChance(baseAttemptChance(best))} при открытии` : best.groupChanceKnown ? `${formatChance(baseAttemptChance(best))} за базовую попытку` : `${formatChance(baseAttemptChance(best))} при выполнении условий`) : '';
    const sourceSummary = best ? [sourceName(best), bestLabel].filter(Boolean).join(' — ') : '';
    state.sourceSections = buildSourceSections(drops);
    state.monsterDrops = null;
    state.monsterWorldDrops = null;

    main.innerHTML = `<section class="page detail-page" data-route="item/${Number(item.id)}">
      ${breadcrumb('items', 'Предметы', item.name)}
      <header class="detail-summary detail-summary--item">
        <div class="detail-heading detail-heading--item"><h1>${escapeHTML(item.name)}</h1><p>${escapeHTML([item.typeLine || item.category, itemLevelSummary(item)].filter(Boolean).join(' · '))}</p><div class="detail-labels">${qualityBadge(item.quality)}${itemClassBadge(presentation.classes)}${itemSetBadge(setMembers.length)}</div></div>
        <button class="favorite-button large ${active ? 'active' : ''}" type="button" data-favorite="${key}" aria-label="${active ? 'Удалить из избранного' : 'Добавить в избранное'}">${icons.star}</button>
      </header>
      ${gameProperties(presentation, 'Характеристики предмета', data.set ? setContent(item, data.set) : '')}
      ${recipeMaterialsHTML(data.recipe)}
      ${chestContentsHTML(data.chest)}
      ${drops.length ? `<section class="source-overview"><div><span class="eyebrow">Лучший источник</span><h2>${escapeHTML(sourceSummary)}</h2><p>${formatCount(drops.length, 'источник', 'источника', 'источников')}</p></div><button class="secondary-button" type="button" data-open-details="item-sources">Показать все источники</button></section>` : ''}
      <section class="detail-accordions">
        ${drops.length ? accordion('Источники получения', formatCount(drops.length, 'вариант', 'варианта', 'вариантов'), sourcesContent(), false, 'item-sources') : ''}
        ${description ? accordion('Описание', '', `<p class="reading-text">${escapeHTML(description)}</p>`, false) : ''}
        ${accordion('Технические сведения', `ID ${item.id}`, `${kvList([...itemTechnicalRows(item), ['Сервер', serverSelect.options[serverSelect.selectedIndex]?.text || state.server]])}`, false)}
      </section>
    </section>`;
    positionSearchWidget(false);
  }

  function effectLabel(option) {
    const spec = state.effectSpecs?.[String(option.type)];
    const name = spec?.name || `Неизвестный эффект (код ${option.type})`;
    const number = Number(option.value);
    const sign = number >= 0 ? '+' : '';
    const value = spec?.percent ? `${sign}${number.toFixed(2).replace('.', ',')}%` : `${sign}${number}`;
    return `${name} ${value}`;
  }

  function setActiveEffectHTML(active) {
    if (!active) return '';
    const trigger = ({ 1: 'умение может', 2: 'исцеление может', 3: 'получение урона может' })[Number(active.state)];
    const rawText = String(active.text || '').replaceAll('\\n', '\n').trim();
    const intro = trigger ? `С ${formatChance(active.chance)} шансом ${trigger}` : `Активный эффект ${active.id}`;
    const body = rawText ? `${intro} ${rawText}` : intro;
    return `<div class="set-effect-line set-effect-line--active">${escapeHTML(body).replaceAll('\n', '<br>')}</div>`;
  }

  function setEffectOptionHTML(option) {
    const tone = Number(option.value) < 0 ? 'penalty' : 'bonus';
    return `<div class="set-effect-line set-effect-line--${tone}">${escapeHTML(effectLabel(option))}</div>`;
  }

  function setEffectsHTML(set) {
    const effects = Array.isArray(set.effects) ? set.effects : [];
    if (!effects.length) return '';
    const thresholds = [];
    const byThreshold = new Map();
    effects.forEach(effect => {
      const required = Number(effect.required);
      if (!byThreshold.has(required)) {
        const group = { required, rows: [] };
        byThreshold.set(required, group);
        thresholds.push(group);
      }
      const group = byThreshold.get(required);
      (effect.options || []).forEach(option => {
        const row = setEffectOptionHTML(option);
        if (row) group.rows.push(row);
      });
      if (effect.active) {
        const row = setActiveEffectHTML(effect.active);
        if (row) group.rows.push(row);
      }
    });
    const visible = thresholds.filter(group => group.rows.some(Boolean));
    if (!visible.length) return '';
    return `<section class="set-effects" aria-labelledby="setEffectsTitle"><h3 id="setEffectsTitle">Эффекты комплекта</h3>${visible.map(group => `<div class="set-effect-threshold"><strong>${formatCount(group.required, 'предмет', 'предмета', 'предметов')}</strong><div>${group.rows.join('')}</div></div>`).join('')}</section>`;
  }

  function setContent(item, set) {
    const members = set.items || [];
    const setName = hasMeaningfulText(set.name) ? `<p class="set-name">${escapeHTML(set.name)}</p>` : '';
    const list = members.length ? `<div class="set-member-list" aria-label="Предметы комплекта">${members.map(member => {
      const current = Number(member.itemId) === Number(item.id);
      return `<a class="set-member-link ${current ? 'current' : ''}" href="#item/${Number(member.itemId)}" ${current ? 'aria-current="page"' : ''}><span>${icons.item}</span><strong>${escapeHTML(member.item)}</strong>${current ? '<small>Текущий</small>' : icons.chevron}</a>`;
    }).join('')}</div>` : '';
    return `<section class="item-inline-set" aria-labelledby="inlineSetTitle"><h2 id="inlineSetTitle">Комплект</h2>${setName}${list}${setEffectsHTML(set)}</section>`;
  }

  function baseAttemptChance(drop) {
    if (!drop) return 0;
    if (!drop.groupChanceKnown) return Number(drop.itemBaseChance ?? drop.itemChance) || 0;
    return Number(drop.baseAttemptChance) || 0;
  }

  function sourceName(drop) {
    const isWorld = drop.source === 'Мировое выпадение';
    const isChest = drop.source === 'Сундук';
    const isQuest = drop.source === 'Квестовый дроп' || drop.source === 'Квестовое выпадение';
    if (isWorld) return drop.monster || 'Мировая добыча';
    if (isChest) return drop.container || 'Сундук';
    if (isQuest) return drop.quest || 'Квестовый источник';
    return `${drop.monster || 'Источник'}${drop.monsterLevel ? ` · Уровень ${drop.monsterLevel}` : ''}`;
  }

  function buildSourceSections(drops) {
    const definitions = [
      { title: 'Точные монстры', sources: ['Выпадение монстра'] },
      { title: 'Мировая добыча', sources: ['Мировое выпадение'] },
      { title: 'Сундуки', sources: ['Сундук'] },
      { title: 'Квестовые источники', sources: ['Квестовый дроп', 'Квестовое выпадение'] },
    ];
    const known = new Set(definitions.flatMap(def => def.sources));
    const result = definitions.map(def => ({ ...def, rows: drops.filter(drop => def.sources.includes(drop.source)), shown: SOURCE_BATCH })).filter(def => def.rows.length);
    const other = drops.filter(drop => !known.has(drop.source));
    if (other.length) result.push({ title: 'Другие источники', sources: [], rows: other, shown: SOURCE_BATCH });
    return result;
  }

  function chestSourceDetails(drop) {
    const variants = Array.isArray(drop.variants) ? drop.variants : [];
    if (variants.length === 1) return chestVariantLabel(variants[0]);
    if (variants.length > 1) return variants.map(variant => {
      const label = chestVariantLabel(variant) || 'обычный вариант';
      return variant.chanceKnown === true ? `${label} — ${formatChance(variant.chance)}` : label;
    }).join(' · ');
    const quantity = Number(drop.quantity) || 0;
    return quantity > 1 ? `×${formatNumber(quantity)}` : '';
  }

  function sourceRow(drop) {
    const isWorld = drop.source === 'Мировое выпадение';
    const isChest = drop.source === 'Сундук';
    const isQuest = drop.source === 'Квестовый дроп' || drop.source === 'Квестовое выпадение';
    const baseDetails = isWorld ? [drop.context] : isChest ? [chestSourceDetails(drop)] : isQuest ? [drop.context] : [drop.slotTitle];
    if (!isQuest && !isChest && drop.groupChanceKnown) {
      baseDetails.push(`Шанс группы: ${formatChance(drop.groupBaseChance ?? drop.groupChance)}`);
      baseDetails.push(`Если группа выбрана: ${formatChance(drop.itemBaseChance ?? drop.itemChance)}`);
    }
    const sourceLabel = sourceName(drop).trim().toLocaleLowerCase('ru-RU');
    const details = [...new Set(baseDetails.filter(Boolean).map(value => String(value).trim()).filter(value => value && value.toLocaleLowerCase('ru-RU') !== sourceLabel))].join(' · ');
    const chanceLabel = isChest ? 'Шанс при открытии' : isQuest ? 'Шанс при выполнении условий' : 'За одну основную попытку';
    const chanceKnown = !isChest || drop.chanceKnown === true;
    const icon = icons[isWorld ? 'home' : isQuest ? 'info' : isChest ? 'item' : 'monster'];
    const chance = chanceKnown ? `<span class="source-chance"><small>${chanceLabel}</small><strong>${formatChance(baseAttemptChance(drop))}</strong></span>` : '';
    const content = `<span class="source-icon">${icon}</span><span><strong>${escapeHTML(sourceName(drop))}</strong>${details ? `<small>${escapeHTML(details)}</small>` : ''}</span>${chance}`;
    if (isWorld) {
      return `<details class="world-source" data-world-source data-item-id="${Number(drop.itemId)}" data-source-line="${Number(drop.sourceLine)}" data-group-id="${Number(drop.groupId)}" data-choice-position="${Number(drop.choicePosition)}" data-item-position="${Number(drop.itemPosition)}"><summary class="source-row">${content}</summary><div class="world-source-results" data-world-source-host><p class="empty-copy">Раскройте источник, чтобы показать подходящих монстров.</p></div></details>`;
    }
    if (isChest && Number(drop.containerId) > 0) return `<a class="source-row" href="#item/${Number(drop.containerId)}">${content}</a>`;
    return drop.monsterId ? `<a class="source-row" href="#monster/${drop.monsterId}">${content}</a>` : `<div class="source-row">${content}</div>`;
  }

  function sourcesContent() {
    const meta = activeServerMeta();
    if (!state.sourceSections.length) return '<p class="empty-copy">Для выбранного сервера источники не найдены.</p>';
    const dates = `<div class="data-inline"><strong>Актуальность данных</strong><span>Обычная добыча: ${formatSourceDate(meta.directDropsUpdatedAt)}</span><span>Состав групп: ${formatSourceDate(meta.dropListsUpdatedAt)}</span><span>Мировая добыча: ${formatSourceDate(meta.worldDropsUpdatedAt)}</span></div>`;
    const help = `<button class="text-button" type="button" data-dialog="chance">${icons.info}<span>Как рассчитывается шанс</span></button>`;
    return `${dates}${help}<div class="source-sections">${state.sourceSections.map((section, index) => sourceSectionHTML(section, index)).join('')}</div>`;
  }

  function sourceSectionHTML(section, index) {
    const visible = section.rows.slice(0, section.shown);
    return `<section class="source-section" data-source-section="${index}"><header><h3>${escapeHTML(section.title)}</h3><span>${formatNumber(section.rows.length)}</span></header><div class="source-list">${visible.map(sourceRow).join('')}</div>${section.shown < section.rows.length ? `<button class="secondary-button load-more" type="button" data-source-more="${index}">Показать ещё ${formatNumber(Math.min(SOURCE_BATCH, section.rows.length - section.shown))}</button>` : ''}</section>`;
  }

  function renderWorldSourceRows(details) {
    const host = details?.querySelector('[data-world-source-host]');
    const monsters = Array.isArray(details?._worldMonsters) ? details._worldMonsters : [];
    if (!host) return;
    const shown = Math.min(monsters.length, Math.max(WORLD_SOURCE_BATCH, Number(details.dataset.worldShown) || WORLD_SOURCE_BATCH));
    const rows = monsters.slice(0, shown).map(monster => `<a class="world-monster-row" href="#monster/${Number(monster.monsterId)}"><span>${icons.monster}</span><span><strong>${escapeHTML(monster.monster || `Монстр ID ${monster.monsterId}`)}</strong>${Number(monster.level) > 0 ? `<small>Уровень ${formatNumber(monster.level)}</small>` : ''}</span><span class="source-chance"><small>За одну основную попытку</small><strong>${formatChance(monster.chance)}</strong></span></a>`).join('');
    const more = shown < monsters.length ? `<button class="secondary-button load-more" type="button" data-world-more>Показать ещё ${formatNumber(Math.min(WORLD_SOURCE_BATCH, monsters.length - shown))}</button>` : '';
    const note = details.dataset.contextMatchKnown === 'false' ? '<p class="world-source-note">В опубликованных данных нет подтверждённой привязки конкретного монстра к типу карты. Список отфильтрован по уровню и типу монстра; условие «Открытая локация/Инстанс» сервер применяет отдельно.</p>' : '';
    host.innerHTML = `${note}${monsters.length ? `<div class="world-monster-list">${rows}</div>${more}` : '<p class="empty-copy">Подходящие монстры по известным условиям не найдены.</p>'}`;
  }

  async function renderWorldSourceMonsters(details) {
    if (!details || details.dataset.worldLoaded === 'true' || details.dataset.worldLoading === 'true') return;
    const host = details.querySelector('[data-world-source-host]');
    if (!host) return;
    details.dataset.worldLoading = 'true';
    host.innerHTML = '<p class="empty-copy">Загрузка подходящих монстров…</p>';
    const params = new URLSearchParams({
      server: state.server,
      itemId: details.dataset.itemId || '',
      sourceLine: details.dataset.sourceLine || '',
      groupId: details.dataset.groupId || '',
      choicePosition: details.dataset.choicePosition || '',
      itemPosition: details.dataset.itemPosition || '',
    });
    try {
      const data = await api(`/api/world-source-monsters?${params.toString()}`, { signal: state.routeController?.signal });
      if (!details.isConnected) return;
      details._worldMonsters = Array.isArray(data?.monsters) ? data.monsters : [];
      details.dataset.worldShown = String(Math.min(WORLD_SOURCE_BATCH, details._worldMonsters.length));
      details.dataset.contextMatchKnown = String(data?.contextMatchKnown !== false);
      details.dataset.worldLoaded = 'true';
      renderWorldSourceRows(details);
    } catch (error) {
      if (error?.name === 'AbortError') return;
      if (details.isConnected) host.innerHTML = '<p class="empty-copy">Не удалось загрузить список монстров. Закройте и снова раскройте источник.</p>';
    } finally {
      delete details.dataset.worldLoading;
    }
  }

  function monsterDetail(data) {
    const monster = data.monster;
    trackRecentlyViewed('monster', monster.id, monster.name);
    const key = `monster:${monster.id}`;
    const active = state.favorites.has(key);
    const presentation = monsterPresentation(monster);
    const description = meaningfulDescription(monster.note, monster.name);
    const slots = data.slots || [];
    const worldRuleCount = Math.max(0, Number(data.worldRuleCount) || 0);
    state.monsterDrops = { slots, groups: [], shellRendered: false };
    state.monsterWorldDrops = { monsterId: Number(monster.id), count: worldRuleCount, slots: [], groups: [], loaded: false, loading: false, shellRendered: false };
    const topDrops = topMonsterDrops(slots, 6);
    main.innerHTML = `<section class="page detail-page" data-route="monster/${Number(monster.id)}">
      ${breadcrumb('monsters', 'Монстры', monster.name)}
      <header class="detail-summary">
        <span class="detail-icon">${icons.monster}</span>
        <div class="detail-heading"><h1>${escapeHTML(monster.name)}</h1><p>${escapeHTML([monster.category, monster.typeName, `Уровень ${monster.level}`].filter(Boolean).join(' · '))}</p><div class="detail-labels">${monster.aggressive ? '<span class="meta-label warning-label">Агрессивный</span>' : '<span class="meta-label">Неагрессивный</span>'}</div></div>
        <button class="favorite-button large ${active ? 'active' : ''}" type="button" data-favorite="${key}" aria-label="${active ? 'Удалить из избранного' : 'Добавить в избранное'}">${icons.star}</button>
      </header>
      ${gameProperties(presentation, 'Характеристики монстра')}
      ${topDrops.length ? `<section class="monster-drop-preview"><header><div><span class="eyebrow">Обычная добыча</span><h2>Основные предметы</h2></div><button class="secondary-button" type="button" data-open-details="monster-drops">Показать всю добычу</button></header><div class="drop-preview-list">${topDrops.map(drop => `<a href="#item/${drop.itemId}"><span>${icons.item}</span><strong>${escapeHTML(drop.item)}</strong><small>${formatChance(drop.chance)} за основную попытку</small></a>`).join('')}</div></section>` : ''}
      <section class="detail-accordions">
        ${slots.length ? accordion('Обычная добыча', formatCount(slots.length, 'вариант', 'варианта', 'вариантов'), `<div data-monster-drops-host><p class="empty-copy">Список будет загружен после раскрытия раздела.</p></div>`, false, 'monster-drops lazy-monster-drops') : ''}
        ${worldRuleCount ? accordion('Мировая добыча', `${formatCount(worldRuleCount, 'правило', 'правила', 'правил')} по уровню и типу`, `<div data-monster-world-drops-host><p class="empty-copy">Список будет загружен после раскрытия раздела.</p></div>`, false, 'monster-world-drops lazy-monster-world-drops') : ''}
        ${description ? accordion('Описание', '', `<p class="reading-text">${escapeHTML(description)}</p>`, false) : ''}
        ${accordion('Технические сведения', `ID ${monster.id}`, `${kvList([...monsterTechnicalRows(monster), ['Сервер', serverSelect.options[serverSelect.selectedIndex]?.text || state.server]])}`, false)}
      </section>
    </section>`;
    positionSearchWidget(false);
  }

  function topMonsterDrops(slots, limit) {
    const byItem = new Map();
    slots.forEach(slot => (slot.choices || []).forEach(choice => (choice.items || []).forEach(item => {
      const chance = Number(item.baseAttemptChance) || 0;
      const previous = byItem.get(item.itemId);
      if (!previous || chance > previous.chance) byItem.set(item.itemId, { ...item, chance });
    })));
    return [...byItem.values()].sort((a, b) => b.chance - a.chance).slice(0, limit);
  }

  function renderMonsterDropShell() {
    const model = state.monsterDrops;
    const host = main.querySelector('[data-monster-drops-host]');
    if (!model || !host || model.shellRendered) return;
    const meta = activeServerMeta();
    const dates = `<div class="data-inline"><strong>Актуальность данных</strong><span>Обычная добыча: ${formatSourceDate(meta.directDropsUpdatedAt)}</span><span>Состав групп: ${formatSourceDate(meta.dropListsUpdatedAt)}</span></div>`;
    if (!model.slots.length) {
      host.innerHTML = `${dates}<p class="empty-copy">Подтверждённая обычная добыча не найдена.</p>`;
      model.shellRendered = true;
      return;
    }
    model.groups = [];
    const slotsHTML = model.slots.map((slot, slotIndex) => {
      const groups = (slot.choices || []).map(choice => {
        const groupIndex = model.groups.length;
        model.groups.push({ choice, shown: 0, rendered: false });
        const count = (choice.items || []).length;
        return `<details class="drop-choice" data-drop-group="${groupIndex}"><summary><span><strong>${escapeHTML(choice.title || 'Группа предметов')}</strong><small>Шанс группы: ${formatChance(choice.baseSelectionChance)} · ${formatCount(count, 'предмет', 'предмета', 'предметов')}</small></span>${icons.chevron}</summary><div class="drop-items" data-drop-group-host="${groupIndex}"><p class="empty-copy">Раскройте группу, чтобы показать предметы.</p></div></details>`;
      }).join('');
      const extraAttempts = [
        slot.addAttempt1Count ? `${formatCount(slot.addAttempt1Count, 'дополнительная попытка', 'дополнительные попытки', 'дополнительных попыток')} при ${formatChance(slot.addAttempt1Rate)}` : '',
        slot.addAttempt2Count ? `${formatCount(slot.addAttempt2Count, 'дополнительная попытка', 'дополнительные попытки', 'дополнительных попыток')} при ${formatChance(slot.addAttempt2Rate)}` : '',
      ].filter(Boolean).join(' · ');
      return `<section class="drop-slot"><header><div><h3>Вариант добычи ${slotIndex + 1}</h3><p>Основная попытка: 1${extraAttempts ? ` · ${extraAttempts}` : ''}</p></div></header><div>${groups || '<p class="empty-copy">Группы не найдены.</p>'}</div></section>`;
    }).join('');
    host.innerHTML = `${dates}<button class="text-button" type="button" data-dialog="chance">${icons.info}<span>Как работает выпадение</span></button><div class="drop-slots">${slotsHTML}</div>`;
    model.shellRendered = true;
  }

  function renderMonsterDropGroup(groupIndex, showAll = false) {
    const group = state.monsterDrops?.groups?.[groupIndex];
    const host = main.querySelector(`[data-drop-group-host="${groupIndex}"]`);
    if (!group || !host) return;
    const items = group.choice.items || [];
    if (showAll) group.shown = items.length;
    else if (!group.rendered) group.shown = Math.min(DROP_BATCH, items.length);
    else group.shown = Math.min(items.length, group.shown + DROP_BATCH);
    group.rendered = true;
    if (!items.length) {
      host.innerHTML = '<p class="empty-copy">Состав группы не найден.</p>';
      return;
    }
    const rows = items.slice(0, group.shown).map(item => `<a href="#item/${item.itemId}"><span>${icons.item}</span><strong>${escapeHTML(item.item)}</strong><small>Если группа выбрана: ${formatChance(item.baseSelectionChance)} · за одну основную попытку: ${formatChance(item.baseAttemptChance)}${formatChanceOdds(item.baseAttemptChance)}${item.quantity > 1 ? ` · ×${item.quantity}` : ''}</small></a>`).join('');
    const remaining = items.length - group.shown;
    host.innerHTML = `${rows}<div class="lazy-list-status" aria-live="polite">Показано ${formatNumber(group.shown)} из ${formatNumber(items.length)}</div>${remaining > 0 ? `<div class="lazy-list-actions"><button class="secondary-button" type="button" data-drop-more="${groupIndex}">Показать ещё ${formatNumber(Math.min(DROP_BATCH, remaining))}</button><button class="text-button" type="button" data-drop-all="${groupIndex}">Показать всё</button></div>` : ''}`;
  }

  function renderMonsterWorldDropSlots() {
    const model = state.monsterWorldDrops;
    const host = main.querySelector('[data-monster-world-drops-host]');
    if (!model || !host) return;
    const meta = activeServerMeta();
    const dates = `<div class="data-inline"><strong>Актуальность данных</strong><span>Состав групп: ${formatSourceDate(meta.dropListsUpdatedAt)}</span><span>Мировая добыча: ${formatSourceDate(meta.worldDropsUpdatedAt)}</span></div>`;
    const note = '<p class="world-source-note">Правила ниже подходят монстру по уровню и типу. Для мирового дропа сервер также учитывает тип локации. В опубликованных данных нет надёжной привязки каждого монстра к открытому миру или инстансу, поэтому это список подходящих правил, а не подтверждение конкретной карты.</p>';
    if (!model.slots.length) {
      host.innerHTML = `${dates}${note}<p class="empty-copy">Подходящие правила мировой добычи не найдены.</p>`;
      model.shellRendered = true;
      return;
    }
    model.groups = [];
    const slotsHTML = model.slots.map((slot, slotIndex) => {
      const groups = (slot.choices || []).map(choice => {
        const groupIndex = model.groups.length;
        model.groups.push({ choice, shown: 0, rendered: false });
        const count = (choice.items || []).length;
        return `<details class="drop-choice" data-monster-world-drop-group="${groupIndex}" data-group-id="${Number(choice.groupId) || 0}"><summary><span><strong>${escapeHTML(choice.title || `Вариант ${groupIndex + 1}`)}</strong><small>Шанс группы: ${formatChance(choice.baseSelectionChance)} · ${formatCount(count, 'предмет', 'предмета', 'предметов')}</small></span>${icons.chevron}</summary><div class="drop-items" data-monster-world-drop-group-host="${groupIndex}"><p class="empty-copy">Раскройте вариант, чтобы показать предметы.</p></div></details>`;
      }).join('');
      const extraAttempts = [
        slot.addAttempt1Count ? `${formatCount(slot.addAttempt1Count, 'дополнительная попытка', 'дополнительные попытки', 'дополнительных попыток')} при ${formatChance(slot.addAttempt1Rate)}` : '',
        slot.addAttempt2Count ? `${formatCount(slot.addAttempt2Count, 'дополнительная попытка', 'дополнительные попытки', 'дополнительных попыток')} при ${formatChance(slot.addAttempt2Rate)}` : '',
      ].filter(Boolean).join(' · ');
      return `<section class="drop-slot"><header><div><h3>${escapeHTML(slot.context || `Правило ${slotIndex + 1}`)}</h3><p>Основная попытка: 1${extraAttempts ? ` · ${extraAttempts}` : ''}</p></div></header><div>${groups || '<p class="empty-copy">Варианты не найдены.</p>'}</div></section>`;
    }).join('');
    host.innerHTML = `${dates}${note}<button class="text-button" type="button" data-dialog="chance">${icons.info}<span>Как работает выпадение</span></button><div class="drop-slots">${slotsHTML}</div>`;
    model.shellRendered = true;
  }

  async function renderMonsterWorldDropShell() {
    const model = state.monsterWorldDrops;
    const host = main.querySelector('[data-monster-world-drops-host]');
    if (!model || !host || model.loading) return;
    if (model.loaded) {
      if (!model.shellRendered) renderMonsterWorldDropSlots();
      return;
    }
    model.loading = true;
    host.innerHTML = '<p class="empty-copy">Загрузка мировой добычи…</p>';
    const params = new URLSearchParams({ server: state.server, monsterId: String(model.monsterId) });
    try {
      const data = await api(`/api/monster-world-drops?${params.toString()}`, { signal: state.routeController?.signal });
      if (!host.isConnected || state.monsterWorldDrops !== model) return;
      model.slots = Array.isArray(data?.slots) ? data.slots : [];
      model.loaded = true;
      model.shellRendered = false;
      renderMonsterWorldDropSlots();
    } catch (error) {
      if (error?.name === 'AbortError') return;
      if (host.isConnected) host.innerHTML = '<p class="empty-copy">Не удалось загрузить мировую добычу. Закройте и снова раскройте раздел.</p>';
    } finally {
      model.loading = false;
    }
  }

  function renderMonsterWorldDropGroup(groupIndex, showAll = false) {
    const group = state.monsterWorldDrops?.groups?.[groupIndex];
    const host = main.querySelector(`[data-monster-world-drop-group-host="${groupIndex}"]`);
    if (!group || !host) return;
    const items = group.choice.items || [];
    if (showAll) group.shown = items.length;
    else if (!group.rendered) group.shown = Math.min(DROP_BATCH, items.length);
    else group.shown = Math.min(items.length, group.shown + DROP_BATCH);
    group.rendered = true;
    if (!items.length) {
      host.innerHTML = '<p class="empty-copy">Состав варианта не найден.</p>';
      return;
    }
    const rows = items.slice(0, group.shown).map(item => `<a href="#item/${item.itemId}"><span>${icons.item}</span><strong>${escapeHTML(item.item)}</strong><small>Если группа выбрана: ${formatChance(item.baseSelectionChance)} · за одну основную попытку: ${formatChance(item.baseAttemptChance)}${formatChanceOdds(item.baseAttemptChance)}${item.quantity > 1 ? ` · ×${item.quantity}` : ''}</small></a>`).join('');
    const remaining = items.length - group.shown;
    host.innerHTML = `${rows}<div class="lazy-list-status" aria-live="polite">Показано ${formatNumber(group.shown)} из ${formatNumber(items.length)}</div>${remaining > 0 ? `<div class="lazy-list-actions"><button class="secondary-button" type="button" data-monster-world-drop-more="${groupIndex}">Показать ещё ${formatNumber(Math.min(DROP_BATCH, remaining))}</button><button class="text-button" type="button" data-monster-world-drop-all="${groupIndex}">Показать всё</button></div>` : ''}`;
  }

  async function favoritesPage(signal) {
    const keys = [...state.favorites];
    if (!keys.length) {
      main.innerHTML = `<section class="page">${pageHeader('Избранное', 'Сохранённые предметы и монстры.')}<div class="state-message compact"><span class="state-symbol">☆</span><h2>Избранное пусто</h2><p>Добавляйте записи кнопкой со звездой.</p><a class="primary-button" href="#items">Открыть предметы</a></div></section>`;
      positionSearchWidget(false);
      return;
    }
    const data = await api('/api/favorites', { method: 'POST', signal, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ keys, server: state.server, page: state.favoritePage, pageSize: FAVORITES_PAGE_SIZE }) });
    state.favoritePage = Math.max(1, Number(data.page || 1));
    const rows = (data.rows || []).map(row => row.kind === 'monster' ? monsterRow(row) : itemRow(row));
    const missing = Number(data.missing || 0);
    main.innerHTML = `<section class="page">${pageHeader('Избранное', `В избранном: ${formatNumber(data.total)}.`)}${missing ? `<p class="muted-copy">Не удалось показать ${formatCount(missing, 'запись', 'записи', 'записей')}. Они остаются сохранены в профиле.</p>` : ''}<div class="result-list">${rows.join('')}</div>${pagination(data.page, data.pages, 'favorite-page')}</section>`;
    positionSearchWidget(false);
  }

  async function renderRoute() {
    closeFilters();
    closeMoreMenu();
    closeSuggestions();
    const previousRoute = state.route;
    const raw = decodeRouteHash();
    const targetPath = raw.split('?')[0];
    const visibleDetail = main.querySelector('.detail-page');
    const visibleCatalog = main.querySelector('.catalog-page');
    const preserveItemDetail = Boolean(visibleDetail?.dataset.route?.startsWith('item/') && targetPath.startsWith('item/'));
    const preserveCatalogPage = Boolean(visibleCatalog && ['items', 'monsters'].includes(targetPath));
    const preserveVisiblePage = preserveItemDetail || preserveCatalogPage;
    const visibleRoute = visibleDetail?.dataset.route || visibleCatalog?.dataset.catalogKind || previousRoute;

    state.routeController?.abort();
    state.catalogController?.abort();
    state.catalog = null;
    if (!preserveItemDetail) {
      state.sourceSections = [];
      state.monsterDrops = null;
      state.monsterWorldDrops = null;
    }
    if (preserveItemDetail) visibleDetail.setAttribute('aria-busy', 'true');
    if (preserveCatalogPage) {
      visibleCatalog.setAttribute('aria-busy', 'true');
      visibleCatalog.setAttribute('inert', '');
    }
    const controller = new AbortController();
    state.routeController = controller;
    const requestId = ++state.requestId;
    const previousBase = routeBase();
    if (targetPath === 'favorites' && previousBase !== 'favorites') state.favoritePage = 1;
    state.route = raw;
    renderNavigation();
    if (!preserveVisiblePage) loadingPage();
    try {
      if (!state.meta) {
        state.meta = await api('/api/meta', { signal: controller.signal });
        state.effectSpecs = state.meta.effectSpecs || {};
        renderServers();
      }
      if (controller.signal.aborted || requestId !== state.requestId) return;
      const [path, queryString = ''] = raw.split('?');
      const params = new URLSearchParams(queryString);
      if (path === 'home') homePage();
      else if (path === 'items' || path === 'monsters') {
        const filters = catalogFilters(path);
        if (params.has('q')) filters.q = params.get('q') || '';
        filters.page = 1;
        const data = await fetchCatalog(path, controller.signal);
        if (controller.signal.aborted || requestId !== state.requestId) return;
        catalogPage(path, data);
      } else if (path === 'search') {
        const query = (params.get('q') || '').trim();
        globalSearch.value = query;
        if (query) await searchPage(query, controller.signal); else homePage();
      } else if (path.startsWith('item/')) {
        const id = path.slice(5);
        const data = await api(`/api/items/${encodeURIComponent(id)}?server=${encodeURIComponent(state.server)}`, { signal: controller.signal });
        if (controller.signal.aborted || requestId !== state.requestId) return;
        itemDetail(data);
      } else if (path.startsWith('monster/')) {
        const id = path.slice(8);
        const data = await api(`/api/monsters/${encodeURIComponent(id)}?server=${encodeURIComponent(state.server)}`, { signal: controller.signal });
        if (controller.signal.aborted || requestId !== state.requestId) return;
        monsterDetail(data);
      } else if (path === 'favorites') await favoritesPage(controller.signal);
      else notFoundPage();
      if (!controller.signal.aborted && requestId === state.requestId) main.focus({ preventScroll: true });
    } catch (error) {
      if (error?.name === 'AbortError') return;
      if (requestId !== state.requestId) return;
      if (preserveVisiblePage) {
        visibleDetail?.removeAttribute('aria-busy');
        visibleCatalog?.removeAttribute('aria-busy');
        visibleCatalog?.removeAttribute('inert');
        state.route = visibleRoute;
        history.replaceState(null, '', `#${visibleRoute}`);
        renderNavigation();
        showToast(preserveItemDetail ? 'Не удалось открыть предмет. Повторите переход.' : 'Не удалось открыть каталог. Повторите переход.');
        return;
      }
      errorPage(error);
    }
  }

  function openMoreMenu() {
    moreMenu.hidden = false;
    moreButton.setAttribute('aria-expanded', 'true');
    requestAnimationFrame(() => moreMenu.querySelector('button, a[href]')?.focus());
  }

  function closeMoreMenu({ restoreFocus = false } = {}) {
    if (moreMenu.hidden) return;
    moreMenu.hidden = true;
    moreButton.setAttribute('aria-expanded', 'false');
    if (restoreFocus) moreButton.focus();
  }

  function updateThemeChrome() {
    themeMenuLabel.textContent = state.theme === 'dark' ? 'Тёмная' : 'Светлая';
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', state.theme === 'dark' ? '#0c111c' : '#f3f5f8');
  }

  function toggleTheme() {
    state.theme = state.theme === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = state.theme;
    localStorage.setItem('iris-theme', state.theme);
    updateThemeChrome();
    scheduleProfileSave(0);
  }

  function openInfoDialog(type) {
    const active = document.activeElement;
    dialogReturnFocus = active?.closest?.('#moreMenu') ? moreButton : active;
    closeMoreMenu();
    if (type === 'about') {
      infoDialogTitle.textContent = 'О приложении';
      infoDialogBody.innerHTML = `<p>Iris Online — локальная база предметов, монстров и источников получения.</p><dl class="kv-list"><div><dt>Версия</dt><dd>${APP_VERSION}</dd></div><div><dt>Автор</dt><dd>Хоуп (Original)</dd></div><div><dt>Работа с данными</dt><dd>Локально на этом компьютере</dd></div></dl><div class="legal-notice"><p><strong>© 2026 Iris Online Database</strong></p><p>Iris Online Database — неофициальное фанатское приложение для Iris Online. Проект не связан с разработчиками, издателями или правообладателями игры. Все игровые материалы, названия, логотипы и товарные знаки принадлежат их соответствующим правообладателям.</p><p><a class="external-link" href="https://irisonline.ru/" target="_blank" rel="noopener noreferrer" aria-label="Официальный сайт игры Iris Online — открыть в новой вкладке">Официальный сайт игры: irisonline.ru ${icons.external}</a></p><p><a class="external-link" href="https://github.com/fsibatov/iris-online-database" target="_blank" rel="noopener noreferrer external" aria-label="GitHub проекта Iris Online Database — открыть в новой вкладке">GitHub проекта ${icons.external}</a></p></div>`;
    } else if (type === 'data') {
      const meta = state.meta?.meta || {};
      const server = activeServerMeta();
      infoDialogTitle.textContent = 'О данных';
      infoDialogBody.innerHTML = `<p>Даты показывают актуальность встроенных данных выбранного сервера.</p><dl class="kv-list"><div><dt>Предметы и характеристики</dt><dd>${formatSourceDate(meta.dataUpdatedAt)}</dd></div><div><dt>Обычная добыча</dt><dd>${formatSourceDate(server.directDropsUpdatedAt)}</dd></div><div><dt>Состав групп</dt><dd>${formatSourceDate(server.dropListsUpdatedAt)}</dd></div><div><dt>Мировая добыча</dt><dd>${formatSourceDate(server.worldDropsUpdatedAt)}</dd></div></dl><p class="muted-copy">Каталог предметов общий. Источники получения зависят от выбранного сервера.</p>`;
    } else if (type === 'feedback') {
      infoDialogTitle.textContent = 'Пожелания и замечания';
      infoDialogBody.innerHTML = `<p>Откройте таблицу Google и оставьте комментарий в нужной ячейке.</p><p><a class="primary-button" target="_blank" rel="noopener noreferrer external" href="https://docs.google.com/spreadsheets/d/1OEKLkfWQPNXG5QXpn1C0JZKOsgxjlTkrE5ckikG4uf4/edit?gid=1073338359#gid=1073338359">Открыть таблицу Google ${icons.external}</a></p>`;
    } else if (type === 'chance') {
      infoDialogTitle.textContent = 'Как работает выпадение';
      infoDialogBody.innerHTML = `<p><strong>Добыча с монстров.</strong> Шанс считается в два простых шага.</p><ol class="chance-steps"><li><strong>Шанс группы.</strong> Сначала игра решает, сработала ли нужная группа наград.</li><li><strong>Если группа выбрана.</strong> Затем игра выбирает конкретный предмет внутри этой группы.</li><li><strong>За одну основную попытку.</strong> Это вероятность, что оба шага сработают подряд.</li></ol><p><strong>Пример:</strong> шанс группы 0,0042%, а предмета внутри неё 0,0833%. Тогда за одну основную попытку шанс предмета около 0,0000035% — примерно 1 из 28,6 млн.</p><p class="muted-copy">Это не обязательно итоговый шанс за убийство. На сервере могут быть дополнительные попытки, ограничения по уровню и времени и другие модификаторы.</p><p><strong>Сундуки.</strong> При открытии сначала определяется набор наград, затем из него выбирается указанное количество предметов. «Шанс при открытии» показывает вероятность получить этот предмет хотя бы один раз за одно открытие. Если точный процент нельзя подтвердить по имеющимся данным, приложение показывает содержимое без процента.</p>`;
    }
    infoDialog.showModal();
    requestAnimationFrame(() => infoDialog.querySelector('[data-close-dialog]')?.focus());
  }

  let profileTimer;
  let profileController;
  let profileDirty = false;
  let profileSaving = false;
  let profileRevision = 0;
  let lastVisibilitySave = 0;
  function persistedFilterValues(filters) {
    const result = {};
    Object.entries(filters).forEach(([key, value]) => { if (key !== 'page' && key !== 'q') result[key] = String(value ?? ''); });
    return result;
  }

  function profilePayload() {
    return {
      schemaVersion: 1,
      migrated: true,
      settings: { server: state.server, theme: state.theme, view: state.view },
      itemFilters: persistedFilterValues(state.itemFilters),
      monsterFilters: persistedFilterValues(state.monsterFilters),
      favorites: [...state.favorites],
      history: state.history.slice(0, 50),
      recentlyViewed: recentViewedEntries(),
    };
  }

  function persistPendingProfile(payload = profilePayload()) {
    try {
      localStorage.setItem(PROFILE_PENDING_KEY, JSON.stringify({ revision: profileRevision, savedAt: Date.now(), profile: payload }));
    } catch (_) {}
  }

  async function saveProfileNow() {
    if (!state.profileLoaded || sessionClosing || profileSaving || !profileDirty) return;
    profileSaving = true;
    const revision = profileRevision;
    const payload = profilePayload();
    profileController = new AbortController();
    try {
      await api('/api/user-data', { method: 'PUT', signal: profileController.signal, body: JSON.stringify(payload), headers: { 'Content-Type': 'application/json' } });
      if (revision === profileRevision) {
        profileDirty = false;
        localStorage.removeItem(PROFILE_PENDING_KEY);
      }
    } catch (_) {
      profileDirty = true;
      persistPendingProfile();
    } finally {
      profileSaving = false;
      profileController = null;
      if (profileDirty && !sessionClosing) {
        clearTimeout(profileTimer);
        profileTimer = setTimeout(saveProfileNow, PROFILE_DEBOUNCE);
      }
    }
  }

  function scheduleProfileSave(delay = PROFILE_DEBOUNCE) {
    localStorage.setItem('iris-history', JSON.stringify(state.history));
    localStorage.setItem('iris-item-filters', JSON.stringify(persistedFilterValues(state.itemFilters)));
    localStorage.setItem('iris-monster-filters', JSON.stringify(persistedFilterValues(state.monsterFilters)));
    profileDirty = true;
    profileRevision += 1;
    persistPendingProfile();
    if (!state.profileLoaded || sessionClosing) return;
    clearTimeout(profileTimer);
    profileTimer = setTimeout(saveProfileNow, delay);
  }

  function saveProfileBestEffort() {
    if (!state.profileLoaded || (!profileDirty && !profileSaving)) return;
    const body = JSON.stringify(profilePayload());
    // Browser keepalive requests share a small transport quota. A large
    // profile has already been scheduled through the normal endpoint and
    // must not prevent the small session-close request from being queued.
    if (new Blob([body]).size > 60 * 1024) return;
    fetch('/api/user-data', {
      method: 'PUT',
      body,
      headers: { 'Content-Type': 'application/json' },
      keepalive: true,
    }).catch(() => {});
  }

  function persistFavorites() {
    localStorage.setItem('iris-favorites', JSON.stringify([...state.favorites]));
    scheduleProfileSave(0);
  }

  function applyFilterProfile(target, source, allowedKeys) {
    if (!source || typeof source !== 'object') return;
    allowedKeys.forEach(key => { if (typeof source[key] === 'string') target[key] = source[key]; });
    target.page = 1;
  }

  async function loadUserProfile() {
    const localRecentlyViewed = safeJSON(localStorage.getItem(RECENT_VIEWED_KEY) || '[]', []);
    const serverProfile = await api('/api/user-data');
    const pending = safeJSON(localStorage.getItem(PROFILE_PENDING_KEY) || 'null', null);
    const pendingProfile = pending && pending.profile && pending.profile.schemaVersion === 1 ? pending.profile : null;
    const profile = pendingProfile || serverProfile;
    if (profile.migrated) {
      state.server = profile.settings?.server || state.server;
      state.theme = profile.settings?.theme || state.theme;
      state.view = profile.settings?.view || state.view;
      state.favorites = new Set(Array.isArray(profile.favorites) ? profile.favorites : []);
      state.history = Array.isArray(profile.history) ? profile.history : [];
      if (Array.isArray(profile.recentlyViewed) && profile.recentlyViewed.length) state.recentlyViewed = profile.recentlyViewed;
      else if (Array.isArray(localRecentlyViewed)) state.recentlyViewed = localRecentlyViewed;
      applyFilterProfile(state.itemFilters, profile.itemFilters, ['category', 'subcategory', 'quality', 'minLevel', 'maxLevel', 'sort']);
      applyFilterProfile(state.monsterFilters, profile.monsterFilters, ['category', 'type', 'minLevel', 'maxLevel', 'sort']);
    } else {
      state.history = safeJSON(localStorage.getItem('iris-history') || '[]', []);
      applyFilterProfile(state.itemFilters, safeJSON(localStorage.getItem('iris-item-filters') || '{}', {}), ['category', 'subcategory', 'quality', 'minLevel', 'maxLevel', 'sort']);
      applyFilterProfile(state.monsterFilters, safeJSON(localStorage.getItem('iris-monster-filters') || '{}', {}), ['category', 'type', 'minLevel', 'maxLevel', 'sort']);
    }
    state.itemFilters.q = '';
    state.monsterFilters.q = '';
    globalSearch.value = '';
    closeSuggestions();
    if (!['list', 'cards'].includes(state.view)) state.view = 'list';
    normalizeDependentFilters('items');
    normalizeDependentFilters('monsters');
    document.documentElement.dataset.theme = state.theme;
    updateThemeChrome();
    localStorage.setItem('iris-server', state.server);
    localStorage.setItem('iris-theme', state.theme);
    localStorage.setItem('iris-view', state.view);
    localStorage.setItem('iris-favorites', JSON.stringify([...state.favorites]));
    localStorage.setItem('iris-item-filters', JSON.stringify(persistedFilterValues(state.itemFilters)));
    localStorage.setItem('iris-monster-filters', JSON.stringify(persistedFilterValues(state.monsterFilters)));
    state.recentlyViewed = recentViewedEntries();
    localStorage.setItem(RECENT_VIEWED_KEY, JSON.stringify(state.recentlyViewed));
    state.profileLoaded = true;
    const migratedLocalRecentlyViewed = state.recentlyViewed.length > 0 && !(Array.isArray(profile.recentlyViewed) && profile.recentlyViewed.length);
    if (pendingProfile || !profile.migrated || migratedLocalRecentlyViewed) scheduleProfileSave(0);
  }

  let heartbeatTimer;
  let sessionOpenController;
  let sessionOpening = false;
  let sessionClosing = false;
  let sessionCloseSent = false;

  function newSessionID() {
    const bytes = new Uint8Array(16);
    if (globalThis.crypto?.getRandomValues) {
      globalThis.crypto.getRandomValues(bytes);
      return [...bytes].map(value => value.toString(16).padStart(2, '0')).join('');
    }
    return `browser-${Date.now()}-${Math.random().toString(36).slice(2, 14)}`;
  }

  function scheduleHeartbeat(delay = 5000) {
    clearTimeout(heartbeatTimer);
    if (!sessionClosing) heartbeatTimer = setTimeout(sendHeartbeat, delay);
  }

  async function sendHeartbeat() {
    if (sessionClosing) return;
    if (!state.sessionId) return openApplicationSession();
    try {
      await api('/api/session/heartbeat', { method: 'POST', body: JSON.stringify({ id: state.sessionId }), headers: { 'Content-Type': 'application/json' } });
      scheduleHeartbeat(5000);
    } catch (_) {
      clearTimeout(heartbeatTimer);
      if (!sessionClosing) heartbeatTimer = setTimeout(openApplicationSession, 2000);
    }
  }

  async function openApplicationSession() {
    if (sessionClosing || sessionOpening) return;
    if (!state.sessionId) state.sessionId = newSessionID();
    sessionOpening = true;
    sessionOpenController = new AbortController();
    try {
      const response = await api('/api/session/open', { method: 'POST', signal: sessionOpenController.signal, body: JSON.stringify({ id: state.sessionId }), headers: { 'Content-Type': 'application/json' } });
      state.sessionId = response.id;
      if (sessionClosing) {
        queueSessionClose(state.sessionId, false);
        return;
      }
      scheduleHeartbeat(Math.max(1000, Number(response.heartbeatSeconds || 5) * 1000));
    } catch (error) {
      if (sessionClosing) return;
      if (error?.status === 409) state.sessionId = '';
      clearTimeout(heartbeatTimer);
      heartbeatTimer = setTimeout(openApplicationSession, 2000);
    } finally {
      sessionOpenController = null;
      sessionOpening = false;
    }
  }

  function abortPendingWork() {
    clearTimeout(suggestionTimer);
    clearTimeout(catalogDebounce);
    clearTimeout(profileTimer);
    clearTimeout(showToast.timer);
    clearTimeout(heartbeatTimer);
    state.routeController?.abort();
    state.catalogController?.abort();
    state.suggestionController?.abort();
    profileController?.abort();
    sessionOpenController?.abort();
  }

  function queueSessionClose(id, pendingOpen = false) {
    if (!id) return;
    const body = JSON.stringify({ id, pendingOpen: Boolean(pendingOpen) });
    const queued = navigator.sendBeacon?.('/api/session/close', new Blob([body], { type: 'application/json' })) === true;
    if (!queued) {
      fetch('/api/session/close', {
        method: 'POST',
        body,
        headers: { 'Content-Type': 'application/json' },
        keepalive: true,
      }).catch(() => {});
    }
  }

  function closeApplicationSession() {
    if (sessionCloseSent) return;
    const pendingOpen = sessionOpening;
    sessionCloseSent = true;
    sessionClosing = true;
    saveProfileBestEffort();
    abortPendingWork();
    queueSessionClose(state.sessionId, pendingOpen);
  }


  main.addEventListener('click', event => {
    const favorite = event.target.closest('[data-favorite]');
    if (favorite) { toggleFavorite(favorite.dataset.favorite, favorite); return; }
    const action = event.target.closest('[data-action]')?.dataset.action;
    if (action === 'reload') { renderRoute(); return; }
    if (action === 'clear-recently-viewed') { clearRecentlyViewed(); return; }
    if (action === 'open-filters') { openFilters(); return; }
    if (action === 'reset-filters') { resetFilters(); return; }
    const pageButton = event.target.closest('[data-page]');
    if (pageButton) {
      catalogFilters(state.catalog.kind).page = Number(pageButton.dataset.page);
      refreshCatalog();
      window.scrollTo({ top: 0, behavior: 'auto' });
      return;
    }
    const favoritePageButton = event.target.closest('[data-favorite-page]');
    if (favoritePageButton) {
      state.favoritePage = Math.max(1, Number(favoritePageButton.dataset.favoritePage) || 1);
      renderRoute();
      window.scrollTo({ top: 0, behavior: 'auto' });
      return;
    }
    const viewButton = event.target.closest('[data-view]');
    if (viewButton) {
      state.view = viewButton.dataset.view;
      localStorage.setItem('iris-view', state.view);
      main.querySelectorAll('[data-view]').forEach(button => button.classList.toggle('active', button === viewButton));
      const data = state.catalog?.data;
      const results = main.querySelector('[data-catalog-results]');
      if (data && results) results.innerHTML = catalogResultsHTML(state.catalog.kind, data);
      scheduleProfileSave();
      return;
    }
    const chip = event.target.closest('[data-clear-filter]');
    if (chip) {
      const filters = catalogFilters(state.catalog.kind);
      filters[chip.dataset.clearFilter] = '';
      normalizeDependentFilters(state.catalog.kind);
      filters.page = 1;
      refreshCatalog({ refreshFilters: true });
      return;
    }
    const historyButton = event.target.closest('[data-history-query]');
    if (historyButton) {
      globalSearch.value = historyButton.dataset.historyQuery;
      submitGlobalSearch();
      return;
    }
    const openDetails = event.target.closest('[data-open-details]');
    if (openDetails) {
      const target = main.querySelector(`.${openDetails.dataset.openDetails}`);
      if (target) { target.open = true; target.scrollIntoView({ block: 'start', behavior: 'smooth' }); target.querySelector('summary')?.focus(); }
      return;
    }
    const sourceMore = event.target.closest('[data-source-more]');
    if (sourceMore) {
      const index = Number(sourceMore.dataset.sourceMore);
      const section = state.sourceSections[index];
      if (!section) return;
      section.shown = Math.min(section.rows.length, section.shown + SOURCE_BATCH);
      const host = main.querySelector(`[data-source-section="${index}"]`);
      if (host) host.outerHTML = sourceSectionHTML(section, index);
      return;
    }
    const worldMore = event.target.closest('[data-world-more]');
    if (worldMore) {
      const details = worldMore.closest('[data-world-source]');
      if (!details || !Array.isArray(details._worldMonsters)) return;
      details.dataset.worldShown = String(Math.min(details._worldMonsters.length, (Number(details.dataset.worldShown) || WORLD_SOURCE_BATCH) + WORLD_SOURCE_BATCH));
      renderWorldSourceRows(details);
      return;
    }
    const dropMore = event.target.closest('[data-drop-more]');
    if (dropMore) {
      renderMonsterDropGroup(Number(dropMore.dataset.dropMore));
      return;
    }
    const dropAll = event.target.closest('[data-drop-all]');
    if (dropAll) {
      renderMonsterDropGroup(Number(dropAll.dataset.dropAll), true);
      return;
    }
    const monsterWorldDropMore = event.target.closest('[data-monster-world-drop-more]');
    if (monsterWorldDropMore) {
      renderMonsterWorldDropGroup(Number(monsterWorldDropMore.dataset.monsterWorldDropMore));
      return;
    }
    const monsterWorldDropAll = event.target.closest('[data-monster-world-drop-all]');
    if (monsterWorldDropAll) {
      renderMonsterWorldDropGroup(Number(monsterWorldDropAll.dataset.monsterWorldDropAll), true);
      return;
    }
    const dialogButton = event.target.closest('[data-dialog]');
    if (dialogButton) openInfoDialog(dialogButton.dataset.dialog);
  });

  main.addEventListener('toggle', event => {
    const details = event.target;
    if (!(details instanceof HTMLDetailsElement) || !details.open) return;
    if (details.matches('.lazy-monster-drops')) renderMonsterDropShell();
    if (details.matches('.lazy-monster-world-drops')) renderMonsterWorldDropShell();
    if (details.matches('[data-drop-group]')) renderMonsterDropGroup(Number(details.dataset.dropGroup));
    if (details.matches('[data-monster-world-drop-group]')) renderMonsterWorldDropGroup(Number(details.dataset.monsterWorldDropGroup));
    if (details.matches('[data-world-source]')) renderWorldSourceMonsters(details);
  }, true);

  main.addEventListener('input', event => {
    if (!event.target.matches('[data-catalog-search]')) return;
    const filters = catalogFilters(state.catalog.kind);
    filters.q = event.target.value;
    filters.page = 1;
    clearTimeout(catalogDebounce);
    catalogDebounce = setTimeout(() => refreshCatalog(), SEARCH_DEBOUNCE);
  });

  main.addEventListener('keydown', event => {
    if (event.target.matches('[data-catalog-search]') && event.key === 'Enter') {
      clearTimeout(catalogDebounce);
      addHistory(event.target.value);
      refreshCatalog();
    }
  });

  main.addEventListener('change', event => {
    if (!event.target.matches('[data-catalog-sort]')) return;
    const filters = catalogFilters(state.catalog.kind);
    filters.sort = event.target.value;
    filters.page = 1;
    refreshCatalog();
  });

  filterDrawerBody.addEventListener('change', event => {
    const input = event.target.closest('[name]');
    if (!input || !state.catalog) return;
    const kind = state.catalog.kind;
    const filters = catalogFilters(kind);
    filters[input.name] = input.value;
    if (input.name === 'category') {
      if (kind === 'items') { filters.subcategory = ''; filters.quality = ''; }
      else filters.type = '';
    }
    filters.page = 1;
    refreshCatalog({ refreshFilters: input.name === 'category' });
  });

  filterDrawerBody.addEventListener('input', event => {
    const input = event.target.closest('input[name="minLevel"], input[name="maxLevel"]');
    if (!input || !state.catalog) return;
    catalogFilters(state.catalog.kind)[input.name] = input.value;
    catalogFilters(state.catalog.kind).page = 1;
    clearTimeout(catalogDebounce);
    catalogDebounce = setTimeout(() => refreshCatalog(), SEARCH_DEBOUNCE);
  });

  resetFiltersButton.addEventListener('click', () => resetFilters());
  closeFiltersButton.addEventListener('click', closeFilters);
  filterDrawer.addEventListener('click', event => { if (event.target.closest('[data-close-overlay]')) closeFilters(); });
  overlayBackdrop.addEventListener('click', closeFilters);

  globalSearch.addEventListener('input', updateSuggestions);
  globalSearch.addEventListener('keydown', event => {
    if (event.key === 'ArrowDown') { event.preventDefault(); if (!suggestions.hidden) setActiveSuggestion(activeSuggestion + 1); }
    else if (event.key === 'ArrowUp') { event.preventDefault(); if (!suggestions.hidden) setActiveSuggestion(activeSuggestion < 0 ? suggestionRoutes.length - 1 : activeSuggestion - 1); }
    else if (event.key === 'Enter') {
      event.preventDefault();
      if (activeSuggestion >= 0 && suggestionRoutes[activeSuggestion]) {
        const route = suggestionRoutes[activeSuggestion];
        addHistory(globalSearch.value);
        closeSuggestions();
        location.hash = route;
      } else submitGlobalSearch();
    } else if (event.key === 'Escape') { closeSuggestions(); }
  });

  suggestions.addEventListener('pointermove', event => {
    const option = event.target.closest('[data-suggestion-index]');
    if (option) setActiveSuggestion(Number(option.dataset.suggestionIndex));
  });
  suggestions.addEventListener('click', event => {
    const option = event.target.closest('[data-suggestion]');
    if (option) {
      addHistory(globalSearch.value);
      closeSuggestions();
      location.hash = option.dataset.suggestion;
      return;
    }
    if (event.target.closest('[data-search-all]')) submitGlobalSearch();
  });

  serverSelect.addEventListener('change', () => {
    const previous = state.server;
    state.server = serverSelect.value;
    localStorage.setItem('iris-server', state.server);
    scheduleProfileSave(0);
    if (previous !== state.server) {
      const serverLabel = serverSelect.options[serverSelect.selectedIndex]?.text || state.server;
      const homeServerName = document.querySelector('[data-home-server-name]');
      if (homeServerName) homeServerName.textContent = serverLabel;
      showToast(`Сервер изменён на ${serverLabel}. Данные обновлены.`);
      if (['favorites'].includes(routeBase()) || state.route.startsWith('item/') || state.route.startsWith('monster/')) renderRoute();
    }
  });

  moreButton.addEventListener('click', () => moreMenu.hidden ? openMoreMenu() : closeMoreMenu({ restoreFocus: true }));
  moreMenu.addEventListener('click', event => {
    const action = event.target.closest('[data-menu-action]')?.dataset.menuAction;
    if (action === 'theme') { toggleTheme(); closeMoreMenu({ restoreFocus: true }); }
    else if (action) openInfoDialog(action);
  });

  document.addEventListener('click', event => {
    if (!event.target.closest('.search-combobox')) closeSuggestions();
    if (!moreMenu.hidden && !event.target.closest('#moreMenu') && !event.target.closest('#moreButton')) closeMoreMenu();
  });

  document.addEventListener('keydown', event => {
    if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey && !['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement?.tagName) && !infoDialog.open) {
      event.preventDefault();
      globalSearch.focus();
    }
    if (event.key === 'Escape') {
      if (!filterDrawer.hidden) { event.preventDefault(); closeFilters(); }
      else if (!moreMenu.hidden) { event.preventDefault(); closeMoreMenu({ restoreFocus: true }); }
    }
    if (!filterDrawer.hidden && event.key === 'Tab') {
      const focusable = [...filterDrawer.querySelectorAll('button:not([disabled]), select:not([disabled]), input:not([disabled]), [href]')];
      if (!focusable.length) return;
      const first = focusable[0], last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    }
  });

  infoDialog.addEventListener('click', event => { if (event.target === infoDialog || event.target.closest('[data-close-dialog]')) infoDialog.close(); });
  infoDialog.addEventListener('close', () => { dialogReturnFocus?.focus?.({ preventScroll: true }); dialogReturnFocus = null; });

  window.addEventListener('hashchange', renderRoute);
  window.addEventListener('pagehide', closeApplicationSession);
  window.addEventListener('pageshow', () => {
    sessionClosing = false;
    sessionCloseSent = false;
    openApplicationSession();
  });
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') {
      const now = Date.now();
      if (now - lastVisibilitySave >= VISIBILITY_SAVE_INTERVAL) {
        lastVisibilitySave = now;
        saveProfileBestEffort();
      }
      return;
    }
    if (!sessionClosing) openApplicationSession();
  });

  (async () => {
    globalSearch.value = '';
    closeSuggestions();
    try {
      await loadUserProfile();
      await openApplicationSession();
    } catch (_) { state.profileLoaded = true; }
    renderRoute();
  })();
})();
