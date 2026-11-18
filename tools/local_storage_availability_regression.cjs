'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.resolve(__dirname, '../web/app.js'), 'utf8');
const start = source.indexOf('  function readLocalSetting(');
const end = source.indexOf('  function defaultItemFilters()', start);
assert.ok(start >= 0 && end > start, 'Storage access helpers are missing');
const helpers = source.slice(start, end);

function load(storage) {
  const context = { localStorage: storage };
  vm.runInNewContext(`${helpers}\nthis.read = readLocalSetting; this.write = writeLocalSetting; this.remove = removeLocalSetting;`, context);
  return context;
}

const failed = load({
  getItem() { throw new Error('SecurityError'); },
  setItem() { throw new Error('QuotaExceededError'); },
  removeItem() { throw new Error('SecurityError'); },
});
assert.equal(failed.read('iris-server', 'kiss'), 'kiss');
assert.equal(failed.read('iris-server'), null);
assert.doesNotThrow(() => failed.write('iris-theme', 'dark'));
assert.doesNotThrow(() => failed.remove('iris-item-filters'));

const values = new Map();
const good = load({
  getItem(key) { return values.has(key) ? values.get(key) : null; },
  setItem(key, value) { values.set(key, value); },
  removeItem(key) { values.delete(key); },
});
good.write('iris-server', 'original');
assert.equal(good.read('iris-server', 'kiss'), 'original');
good.remove('iris-server');
assert.equal(good.read('iris-server', 'kiss'), 'kiss');

for (const key of ['iris-favorites', 'iris-server', 'iris-theme', 'iris-view']) {
  assert.ok(source.includes(`readLocalSetting('${key}')`), `Unprotected startup read: ${key}`);
}
assert.ok(source.includes('readLocalSetting(RECENT_VIEWED_KEY)'), 'Recent-viewed startup read is unprotected');
assert.ok(source.includes('readLocalSetting(PROFILE_PENDING_KEY)'), 'Pending-profile read is unprotected');
console.log('Local storage access denied/quota regression: PASS');
