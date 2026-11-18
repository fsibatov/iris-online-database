'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.resolve(__dirname, '../web/app.js'), 'utf8');
const recentStart = source.indexOf('  function normalizedRecentViewedEntries() {');
const recentEnd = source.indexOf('  function homePage() {', recentStart);
const profileStart = source.indexOf('  function scheduleProfileSave(');
const profileEnd = source.indexOf('  function saveProfileBestEffort()', profileStart);
assert(recentStart >= 0 && recentEnd > recentStart && profileStart >= 0 && profileEnd > profileStart);
const state = { recentlyViewed: [], server: 'kiss', profileLoaded: true, history: [] };
let timerCalls = 0;
const context = {
  state,
  localStorage: { setItem() { throw new Error('QuotaExceededError'); } },
  RECENT_VIEWED_KEY: 'test-recent', RECENT_VIEWED_LIMIT: 8, PROFILE_DEBOUNCE: 400,
  profileDirty: false, profileRevision: 0, profileTimer: null, applicationClosing: false,
  persistPendingProfile() {}, saveProfileNow() {},
  setTimeout() { timerCalls++; return 1; }, clearTimeout() {},
  hasMeaningfulText: text => typeof text === 'string' && Boolean(text.trim()),
  normalizeServerKey: key => key === 'or' ? 'original' : key,
  routeBase: () => 'items', showToast() {},
};
vm.runInNewContext(`${source.slice(recentStart, recentEnd)}\n${source.slice(profileStart, profileEnd)}\nthis.track = trackRecentlyViewed; this.clear = clearRecentlyViewed;`, context);
context.track('item', 42, 'Example');
assert.equal(state.recentlyViewed.length, 1);
assert.equal(state.recentlyViewed[0].id, 42);
assert.equal(timerCalls, 1, 'profile save must still be scheduled');
context.clear();
assert.equal(state.recentlyViewed.length, 0);
assert.equal(timerCalls, 2);
console.log('Recently-viewed storage quota resilience: PASS');
