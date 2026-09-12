import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import { readLocalValue, writeLocalValue, removeLocalValue, profileRetryDelay } from '../web/profile.js';

const script = readFileSync(new URL('../web/app.js', import.meta.url), 'utf8');
const styles = readFileSync(new URL('../web/styles.css', import.meta.url), 'utf8');

function sourceFunction(name) {
  const match = script.match(new RegExp(`^  (?:async )?function ${name}\\([^]*?^  }`, 'm'));
  assert.ok(match, name);
  return match[0];
}

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

function timerQueue() {
  let sequence = 0;
  const timers = new Map();
  return {
    timers,
    setTimeout(fn, delay) { const id = ++sequence; timers.set(id, { fn, delay }); return id; },
    clearTimeout(id) { timers.delete(id); },
    async next() {
      assert.equal(timers.size, 1);
      const [id, timer] = timers.entries().next().value;
      timers.delete(id);
      await timer.fn();
      return timer.delay;
    },
  };
}

function profileFixture() {
  const clock = timerQueue();
  const notice = { hidden: true };
  const drafts = new Map();
  const writes = [];
  const context = {
    ...clock, AbortController, PROFILE_DEBOUNCE: 400, PROFILE_PENDING_KEY: 'pending',
    state: { profileLoaded: true, history: [] }, applicationClosing: false,
    profileTimer: null, profileController: null, profileDirty: true, profileSaving: false,
    profileRevision: 1, profileFailures: 0, profileFailureStatus: 0, profileRetryDelay,
    document: { getElementById() { return notice; } },
    writeLocalValue(key, value) { drafts.set(key, value); return true; },
    removeLocalValue(key) { drafts.delete(key); },
    profilePayload() { return { revision: context.profileRevision }; },
    api(_path, options) { writes.push(JSON.parse(options.body)); return context.write(options); },
    write() { return Promise.reject(new Error('write denied')); },
  };
  vm.createContext(context);
  for (const name of ['showProfileSaveFailure', 'persistPendingProfile', 'queueProfileSave', 'saveProfileNow', 'scheduleProfileSave', 'retryProfileSave']) {
    vm.runInContext(sourceFunction(name), context);
  }
  return { context, clock, notice, drafts, writes };
}

test('profile retries stop after bounded backoff and manual retry recovers', async () => {
  const { context, clock, notice, drafts, writes } = profileFixture();
  await context.saveProfileNow();
  const delays = [];
  while (clock.timers.size) delays.push(await clock.next());
  assert.deepEqual(delays, [1000, 2000, 4000, 8000, 16000]);
  assert.equal(writes.length, 6);
  assert.equal(context.profileDirty, true);
  assert.equal(notice.hidden, false);
  assert.ok(drafts.has('pending'));
  context.scheduleProfileSave(0);
  assert.equal(clock.timers.size, 0);
  context.write = () => Promise.resolve();
  context.retryProfileSave();
  assert.equal(await clock.next(), 0);
  assert.equal(context.profileDirty, false);
  assert.equal(notice.hidden, true);
  assert.equal(drafts.has('pending'), false);
});

test('permanent profile errors require an explicit retry', async () => {
  for (const status of [400, 403, 413, 422]) {
    const { context, clock, notice } = profileFixture();
    context.write = () => Promise.reject(Object.assign(new Error('invalid'), { status }));
    await context.saveProfileNow();
    assert.equal(clock.timers.size, 0, String(status));
    assert.equal(context.profileDirty, true);
    assert.equal(notice.hidden, false);
  }
});

test('a profile edit during a write is saved in a subsequent revision', async () => {
  const { context, clock, drafts, writes } = profileFixture();
  const pending = deferred();
  context.write = () => pending.promise;
  const saving = context.saveProfileNow();
  context.scheduleProfileSave(0);
  pending.resolve();
  await saving;
  assert.equal(context.profileDirty, true);
  assert.ok(drafts.has('pending'));
  context.write = () => Promise.resolve();
  await clock.next();
  assert.deepEqual(writes, [{ revision: 1 }, { revision: 2 }]);
  assert.equal(context.profileDirty, false);
});

test('window shutdown preserves the pending profile without scheduling work', async () => {
  const { context, clock, drafts } = profileFixture();
  context.write = ({ signal }) => new Promise((_resolve, reject) => signal.addEventListener('abort', () => reject(new DOMException('closing', 'AbortError'))));
  const saving = context.saveProfileNow();
  context.applicationClosing = true;
  context.profileController.abort();
  await saving;
  assert.equal(clock.timers.size, 0);
  assert.equal(context.profileDirty, true);
  assert.ok(drafts.has('pending'));
});

test('blocked local storage does not break profile loading or writing', () => {
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, get() { throw new Error('blocked'); } });
  try {
    assert.equal(readLocalValue('profile'), null);
    assert.equal(writeLocalValue('profile', '{}'), false);
    assert.equal(removeLocalValue('profile'), false);
  } finally {
    if (descriptor) Object.defineProperty(globalThis, 'localStorage', descriptor);
    else delete globalThis.localStorage;
  }
});

function routeFixture(route) {
  let hash = route;
  const filters = { page: 8, knownSource: '1', q: 'меч', sort: 'name' };
  const requests = [], rendered = [], focused = [], errors = [];
  const page = (route, kind) => ({
    dataset: { route, catalogKind: kind }, attributes: new Map(),
    setAttribute(key, value) { this.attributes.set(key, value); if (key === 'data-route') this.dataset.route = value; },
    removeAttribute(key) { this.attributes.delete(key); },
  });
  const noop = () => {};
  const context = {
    AbortController, URLSearchParams, applicationClosing: false,
    routeScrollPositions: new Map(), routeHistoryIndex: 0, page: null,
    window: { scrollY: 0, scrollTo({ top }) { this.scrollY = top; } },
    state: { route, server: 'kiss', meta: {}, requestId: 0 },
    serverSelect: { value: 'original', options: [{ text: 'The Original' }], selectedIndex: 0, focus() { focused.push(context.state.server); } },
    normalizeServerKey: value => value,
    writeLocalValue: noop, scheduleProfileSave: noop, showToast: noop,
    closeFilters: noop, closeMoreMenu: noop, closeSuggestions: noop,
    routeBase: () => context.state.route.split('?')[0],
    decodeRouteHash: () => hash,
    replaceRouteHash(value) { hash = value; },
    catalogFilters: () => filters,
    renderNavigation: noop, loadingPage() { context.page = null; },
    homePage() { context.page = page('home'); },
    main: { querySelector(selector) { return selector === '.page' || (selector === '.catalog-page' && context.page?.dataset.catalogKind) ? context.page : null; }, focus: noop },
    catalogPage(kind, data) { rendered.push({ kind, data }); context.state.catalog = { kind, data }; context.page = page(context.state.route, kind); },
    fetchCatalog(kind, signal) {
      const request = { kind, signal, server: context.state.server, ...deferred() };
      requests.push(request);
      return request.promise;
    },
    errorPage(error) { errors.push(error); context.page = page('error'); },
    notFoundPage() { rendered.push({ missing: true }); },
  };
  vm.createContext(context);
  vm.runInContext(sourceFunction('renderRoute'), context);
  vm.runInContext(sourceFunction('changeServer'), context);
  return { context, filters, requests, rendered, focused, errors, page };
}

test('catalogue transitions retain the page until the response is ready', async () => {
  const { context, page, requests } = routeFixture('items?q=меч&page=8');
  const visible = page(context.state.route, 'items');
  context.page = visible;
  context.state.catalog = { kind: 'items', data: { total: 20 } };
  context.replaceRouteHash('monsters');
  const opening = context.renderRoute();
  assert.equal(context.page, visible);
  assert.equal(visible.attributes.get('aria-busy'), 'true');
  assert.equal(visible.attributes.has('inert'), true);
  requests[0].resolve({ total: 7 });
  await opening;
  assert.notEqual(context.page, visible);
  assert.equal(context.page.dataset.route, 'monsters');
});

test('back navigation restores the saved scroll position after rendering', async () => {
  const { context, requests } = routeFixture('item/42');
  context.routeScrollPositions.set(3, 540);
  context.routeHistoryIndex = 3;
  context.replaceRouteHash('items?page=8');
  const opening = context.renderRoute();
  requests[0].resolve({ total: 30 });
  await opening;
  assert.equal(context.window.scrollY, 540);
});

test('a failed route restores the previous catalogue and its full address', async () => {
  const { context, page, requests } = routeFixture('items?q=меч&page=8');
  const catalog = { kind: 'items', data: { total: 20 } };
  const visible = page(context.state.route, 'items');
  context.page = visible;
  context.state.catalog = catalog;
  context.replaceRouteHash('monsters');
  const opening = context.renderRoute();
  requests[0].reject(new Error('offline'));
  await opening;
  assert.equal(context.page, visible);
  assert.equal(context.state.catalog, catalog);
  assert.equal(context.decodeRouteHash(), 'items?q=меч&page=8');
  assert.equal(visible.attributes.has('inert'), false);
  assert.equal(visible.attributes.has('aria-busy'), false);
});

test('a failed server switch cannot leave previous server data on screen', async () => {
  const { context, page, requests, errors } = routeFixture('items');
  const visible = page('items', 'items');
  context.page = visible;
  context.state.catalog = { kind: 'items', data: { server: 'kiss' } };
  const changing = context.changeServer();
  assert.equal(context.page, visible);
  requests[0].reject(new Error('offline'));
  await changing;
  assert.notEqual(context.page, visible);
  assert.equal(context.state.catalog, null);
  assert.equal(errors.length, 1);
});

test('scroll history stays bounded and refreshes recently used entries', () => {
  const { context } = routeFixture('home');
  vm.runInContext(sourceFunction('rememberRouteScroll'), context);
  for (let index = 0; index < 70; index += 1) {
    context.routeHistoryIndex = index;
    context.window.scrollY = index * 10;
    context.rememberRouteScroll();
  }
  assert.equal(context.routeScrollPositions.size, 50);
  assert.equal(context.routeScrollPositions.has(19), false);
  context.routeHistoryIndex = 20;
  context.window.scrollY = 730;
  context.rememberRouteScroll();
  assert.equal(context.routeScrollPositions.get(20), 730);
  assert.equal([...context.routeScrollPositions.keys()].at(-1), 20);
});

test('server changes refresh every catalogue and preserve filters', async () => {
  for (const kind of ['items', 'recipes', 'titles', 'monsters', 'transformations']) {
    const { context, filters, requests, rendered, focused } = routeFixture(`${kind}?page=8`);
    const changing = context.changeServer();
    assert.equal(requests.length, 1, kind);
    assert.equal(requests[0].server, 'original');
    assert.equal(filters.page, 1);
    assert.equal(filters.knownSource, '1');
    assert.equal(filters.q, 'меч');
    requests[0].resolve({ server: 'original', total: 5581 });
    await changing;
    assert.equal(rendered[0].data.server, 'original');
    assert.deepEqual(focused, ['original']);
  }
});

test('a late catalogue reply cannot restore the previous server', async () => {
  const { context, requests, rendered, focused } = routeFixture('items');
  const first = context.changeServer();
  context.serverSelect.value = 'kiss';
  const second = context.changeServer();
  assert.equal(requests[0].signal.aborted, true);
  requests[1].resolve({ server: 'kiss', total: 6824 });
  await second;
  requests[0].resolve({ server: 'original', total: 5581 });
  await first;
  assert.equal(rendered.length, 1);
  assert.equal(rendered[0].data.server, 'kiss');
  assert.deepEqual(focused, ['kiss']);
});

test('battleground timer pauses when hidden and resumes once', () => {
  const clock = timerQueue();
  let renders = 0;
  const context = {
    ...clock, window: clock, battlegroundTimer: null, applicationClosing: false,
    document: { visibilityState: 'hidden' }, renderBattlegroundStatus() { renders += 1; },
  };
  vm.createContext(context);
  vm.runInContext(sourceFunction('scheduleBattlegroundStatus'), context);
  context.scheduleBattlegroundStatus();
  assert.equal(clock.timers.size, 0);
  assert.equal(renders, 0);
  context.document.visibilityState = 'visible';
  context.scheduleBattlegroundStatus();
  context.scheduleBattlegroundStatus();
  assert.equal(clock.timers.size, 1);
  context.applicationClosing = true;
  context.scheduleBattlegroundStatus();
  assert.equal(clock.timers.size, 0);
});

test('battleground name changes with the event and the countdown keeps its own label', () => {
  const attributes = new Map();
  const context = {
    battlegroundStatus: { title: '', getAttribute: key => attributes.get(key), setAttribute: (key, value) => attributes.set(key, value) },
    battlegroundName: { textContent: '' }, battlegroundCountdown: { textContent: '' },
    next: null,
  };
  context.battlegroundState = () => context.next;
  vm.createContext(context);
  vm.runInContext(sourceFunction('renderBattlegroundStatus'), context);
  for (const name of ['Противостояние', 'Захват флага', 'Горнило']) {
    context.next = { name, countdown: '29:59' };
    context.renderBattlegroundStatus();
    assert.equal(context.battlegroundName.textContent, name);
    assert.equal(context.battlegroundCountdown.textContent, '29:59');
    assert.equal(attributes.get('aria-label'), `${name}: до начала 29:59`);
    assert.equal(context.battlegroundStatus.title, '');
    assert.doesNotMatch(attributes.get('aria-label'), /БГ|UTC/);
  }
});

test('version status keeps compact text and full update details accessible', () => {
  const attributes = new Map(), buttonAttributes = new Map();
  const context = {
    APP_VERSION: '2.0.6', state: { updateInfo: {} },
    versionStatus: { title: '', dataset: {}, setAttribute: (key, value) => attributes.set(key, value) },
    versionStatusText: { textContent: '' },
    checkUpdatesButton: { disabled: false, setAttribute: (key, value) => buttonAttributes.set(key, value) },
    updateFailureMessage: () => 'Нет подключения к интернету',
  };
  vm.createContext(context);
  vm.runInContext(sourceFunction('renderVersionStatus'), context);
  for (const [info, status, text, detail] of [
    [{}, 'unknown', 'Не проверена', 'Не проверена'],
    [{ checking: true }, 'checking', 'Проверка…', 'Проверка…'],
    [{ checked: true }, 'current', 'Актуальная', 'Актуальная'],
    [{ checked: true, updateAvailable: true, latestVersion: '200.100.300' }, 'update', 'Обновление', 'Доступна версия 200.100.300'],
    [{ checked: true, updateAvailable: true }, 'update', 'Обновление', 'Есть обновление'],
    [{ checked: true, stale: true, failure: 'offline' }, 'stale', 'Проверить', 'Нет подключения к интернету. Показан результат последней успешной проверки'],
    [{ failure: 'offline' }, 'error', 'Не проверена', 'Нет подключения к интернету'],
  ]) {
    context.state.updateInfo = info;
    context.renderVersionStatus();
    assert.equal(context.versionStatus.dataset.status, status);
    assert.equal(context.versionStatusText.textContent, text);
    assert.equal(attributes.get('aria-label'), `Версия 2.0.6. ${detail}`);
    assert.equal(context.versionStatus.title, '');
    assert.equal(context.checkUpdatesButton.disabled, Boolean(info.checking));
    assert.equal(buttonAttributes.get('aria-busy'), info.checking ? 'true' : 'false');
  }
});

function luminance(hex) {
  const values = hex.match(/[a-f\d]{2}/gi).map(value => Number.parseInt(value, 16) / 255).map(value => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4);
  return values[0] * .2126 + values[1] * .7152 + values[2] * .0722;
}

function overlayFixture(gutter = 17, viewportGutter = gutter) {
  const classes = new Set(), properties = new Map();
  const background = { setAttribute() {}, removeAttribute() {} };
  const root = {
    get clientWidth() { return Math.round(context.window.innerWidth - (classes.has('overlay-open') || context.infoDialog.open ? 0 : viewportGutter)); },
    classList: { contains: value => classes.has(value), add: value => classes.add(value), remove: value => classes.delete(value) },
    style: { setProperty: (key, value) => properties.set(key, value), removeProperty: key => properties.delete(key) },
  };
  const context = {
    document: {
      documentElement: root, activeElement: null, querySelector: () => background,
      body: {
        getBoundingClientRect() {
          const reservedWidth = classes.has('overlay-open') || context.infoDialog.open ? 0 : gutter;
          return { width: context.window.innerWidth - reservedWidth - Number.parseFloat(properties.get('--overlay-scrollbar-width') || '0') };
        },
      },
    },
    window: { innerWidth: 320 }, filterDrawer: { hidden: true },
    infoDialog: { open: false, showModal() { this.open = true; }, querySelector: () => null },
    infoDialogTitle: {}, infoDialogBody: {}, dialogReturnFocus: null, icons: { external: '' },
    closeMoreMenu() {}, requestAnimationFrame: callback => callback(),
  };
  vm.createContext(context);
  for (const name of ['setOverlayScrollLocked', 'setBackgroundInert', 'openInfoDialog']) {
    vm.runInContext(sourceFunction(name), context);
  }
  return { context, classes, properties };
}

test('scroll locking preserves measured scrollbar space across repeated locks', () => {
  for (const gutter of [0, 12.5, 15, 17]) {
    const { context, classes, properties } = overlayFixture(gutter);
    context.setOverlayScrollLocked(true);
    assert.equal(properties.get('--overlay-scrollbar-width'), `${gutter}px`);
    assert.equal(classes.has('overlay-open'), true);
    context.setOverlayScrollLocked(true);
    assert.equal(properties.get('--overlay-scrollbar-width'), `${gutter}px`);
    context.setOverlayScrollLocked(false);
    assert.equal(properties.size, 0);
    assert.equal(classes.has('overlay-open'), false);
    context.setOverlayScrollLocked(false);
    assert.equal(properties.size, 0);
  }
});

test('scroll locking preserves a reserved gutter when the viewport reports no scrollbar', () => {
  const { context, classes, properties } = overlayFixture(15, 0);
  const widthBefore = context.document.body.getBoundingClientRect().width;
  assert.equal(widthBefore, 305);
  assert.equal(context.window.innerWidth - context.document.documentElement.clientWidth, 0);
  context.openInfoDialog('feedback');
  assert.equal(properties.get('--overlay-scrollbar-width'), '15px');
  assert.equal(context.document.body.getBoundingClientRect().width, widthBefore);
  context.infoDialog.open = false;
  context.setOverlayScrollLocked(false);
  assert.equal(classes.has('overlay-open'), false);
  assert.equal(properties.size, 0);
  assert.equal(context.document.body.getBoundingClientRect().width, widthBefore);
});

test('scroll locking keeps the page responsive and remeasures after closing', () => {
  const { context, properties } = overlayFixture(15);
  context.setOverlayScrollLocked(true);
  context.window.innerWidth = 380;
  assert.equal(context.document.body.getBoundingClientRect().width, 365);
  context.setOverlayScrollLocked(false);
  assert.equal(context.document.body.getBoundingClientRect().width, 365);
  context.setOverlayScrollLocked(true);
  assert.equal(properties.get('--overlay-scrollbar-width'), '15px');
  assert.equal(context.document.body.getBoundingClientRect().width, 365);
});

test('opening a native dialog measures the gutter before native layout changes', () => {
  const { context, classes, properties } = overlayFixture();
  context.openInfoDialog('feedback');
  assert.equal(context.infoDialog.open, true);
  assert.equal(properties.get('--overlay-scrollbar-width'), '17px');
  context.setBackgroundInert(false);
  assert.equal(classes.has('overlay-open'), true);
  assert.equal(properties.get('--overlay-scrollbar-width'), '17px');
  context.infoDialog.open = false;
  context.setBackgroundInert(false);
  assert.equal(classes.has('overlay-open'), false);
  assert.equal(properties.size, 0);
});

test('a native dialog failure releases only its own scroll lock', () => {
  for (const drawerOpen of [false, true]) {
    const { context, classes, properties } = overlayFixture();
    context.filterDrawer.hidden = !drawerOpen;
    if (drawerOpen) context.setBackgroundInert(true);
    context.infoDialog.showModal = () => { throw new Error('cannot open'); };
    assert.throws(() => context.openInfoDialog('feedback'), /cannot open/);
    assert.equal(classes.has('overlay-open'), drawerOpen);
    assert.equal(properties.get('--overlay-scrollbar-width'), drawerOpen ? '17px' : undefined);
  }
});

test('rarity colours are unchanged and readable on the light-theme badge', () => {
  const background = styles.match(/html\[data-theme="light"\] \.rarity-label:not\(\.quality-default\)\s*\{\s*background:\s*(#[a-f\d]{6})/i)?.[1];
  assert.ok(background);
  const colours = { normal: '#ffffff', unique: '#fff600', epic: '#d800ff', rare: '#00fffc', magic: '#00ff00', shop: '#ffcd00', event: '#c9a0dc' };
  for (const [quality, expected] of Object.entries(colours)) {
    const actual = styles.match(new RegExp(`\\.rarity-label\\.quality-${quality}\\s*\\{[^}]*?color:\\s*(#[a-f\\d]{6})`, 'i'))?.[1];
    assert.equal(actual, expected);
    const ratio = (luminance(actual) + .05) / (luminance(background) + .05);
    assert.ok(ratio >= 4.5, `${quality}: ${ratio}`);
  }
});

test('every CSS variable reference has a definition', () => {
  const defined = new Set([...styles.matchAll(/(--[\w-]+)\s*:/g)].map(match => match[1]));
  const referenced = new Set([...styles.matchAll(/var\((--[\w-]+)/g)].map(match => match[1]));
  assert.deepEqual([...referenced].filter(name => !defined.has(name)), []);
});
